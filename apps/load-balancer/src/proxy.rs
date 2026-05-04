use std::io;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;

use memchr::memmem;
use socket2::{Domain, Protocol, Socket, Type};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream};
use tokio::time::timeout;

use crate::round_robin::RoundRobin;
use crate::upstream::UpstreamPool;

const BUFFER_SIZE: usize = 16 * 1024;
const CLIENT_READ_TIMEOUT: Duration = Duration::from_millis(2000);
const UPSTREAM_IO_TIMEOUT: Duration = Duration::from_millis(2000);
const BAD_GATEWAY: &[u8] =
    b"HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n";

#[derive(Debug)]
pub struct Proxy {
    upstreams: Vec<Arc<UpstreamPool>>,
    balancer: RoundRobin,
}

impl Proxy {
    pub fn new(upstreams: Vec<Arc<UpstreamPool>>) -> Self {
        let balancer = RoundRobin::new(upstreams.len());
        Self {
            upstreams,
            balancer,
        }
    }

    pub async fn serve(self: Arc<Self>, bind_addr: &str) -> io::Result<()> {
        let listener = bind(bind_addr)?;
        loop {
            let (client, _) = listener.accept().await?;
            client.set_nodelay(true)?;
            let proxy = Arc::clone(&self);
            tokio::spawn(async move {
                let _ = proxy.handle_client(client).await;
            });
        }
    }

    async fn handle_client(self: Arc<Self>, mut client: TcpStream) -> io::Result<()> {
        let mut request = Vec::with_capacity(BUFFER_SIZE);
        let mut response = Vec::with_capacity(BUFFER_SIZE);

        loop {
            request.clear();
            response.clear();
            if !read_http_message(&mut client, &mut request, CLIENT_READ_TIMEOUT).await? {
                return Ok(());
            }

            if self
                .forward_with_retry(&request, &mut response)
                .await
                .is_err()
            {
                let _ = write_all_timeout(&mut client, BAD_GATEWAY).await;
                return Ok(());
            }

            write_all_timeout(&mut client, &response).await?;
        }
    }

    async fn forward_with_retry(&self, request: &[u8], response: &mut Vec<u8>) -> io::Result<()> {
        let first = self.balancer.next();
        match self.forward_once(first, request, response).await {
            Ok(()) => Ok(()),
            Err(err) if self.upstreams.len() > 1 => {
                response.clear();
                let second = (first + 1) % self.upstreams.len();
                self.forward_once(second, request, response)
                    .await
                    .map_err(|_| err)
            }
            Err(err) => Err(err),
        }
    }

    async fn forward_once(
        &self,
        upstream_idx: usize,
        request: &[u8],
        response: &mut Vec<u8>,
    ) -> io::Result<()> {
        let pool = &self.upstreams[upstream_idx];
        let mut upstream = pool.acquire().await?;

        if write_all_timeout(&mut upstream, request).await.is_err() {
            return Err(io::Error::new(
                io::ErrorKind::BrokenPipe,
                "upstream write failed",
            ));
        }

        if read_http_message(&mut upstream, response, UPSTREAM_IO_TIMEOUT)
            .await
            .is_err()
        {
            return Err(io::Error::new(
                io::ErrorKind::UnexpectedEof,
                "upstream read failed",
            ));
        }

        pool.release(upstream);
        Ok(())
    }
}

async fn read_http_message(
    stream: &mut TcpStream,
    out: &mut Vec<u8>,
    read_timeout: Duration,
) -> io::Result<bool> {
    let mut header_end = None;
    while header_end.is_none() {
        let n = read_some_timeout(stream, out, read_timeout).await?;
        if n == 0 {
            return Ok(false);
        }
        header_end = find_header_end(out);
    }

    let body_start = header_end.expect("header exists") + 4;
    let content_length = parse_content_length(&out[..body_start]);
    while out.len().saturating_sub(body_start) < content_length {
        let n = read_some_timeout(stream, out, read_timeout).await?;
        if n == 0 {
            return Err(io::Error::new(
                io::ErrorKind::UnexpectedEof,
                "message ended early",
            ));
        }
    }

    Ok(true)
}

async fn read_some_timeout(
    stream: &mut TcpStream,
    out: &mut Vec<u8>,
    read_timeout: Duration,
) -> io::Result<usize> {
    let mut buffer = [0_u8; BUFFER_SIZE];
    let n = timeout(read_timeout, stream.read(&mut buffer)).await??;
    if n > 0 {
        out.extend_from_slice(&buffer[..n]);
    }
    Ok(n)
}

async fn write_all_timeout(stream: &mut TcpStream, bytes: &[u8]) -> io::Result<()> {
    timeout(UPSTREAM_IO_TIMEOUT, stream.write_all(bytes)).await??;
    Ok(())
}

fn bind(bind_addr: &str) -> io::Result<TcpListener> {
    let addr = bind_addr.parse::<SocketAddr>().map_err(|err| {
        io::Error::new(
            io::ErrorKind::InvalidInput,
            format!("invalid bind address: {err}"),
        )
    })?;
    let domain = if addr.is_ipv4() {
        Domain::IPV4
    } else {
        Domain::IPV6
    };
    let socket = Socket::new(domain, Type::STREAM, Some(Protocol::TCP))?;
    socket.set_reuse_address(true)?;
    socket.set_nonblocking(true)?;
    socket.bind(&addr.into())?;
    socket.listen(4096)?;
    TcpListener::from_std(socket.into())
}

fn find_header_end(bytes: &[u8]) -> Option<usize> {
    memmem::find(bytes, b"\r\n\r\n")
}

fn parse_content_length(header: &[u8]) -> usize {
    for line in header.split(|byte| *byte == b'\n') {
        let line = line.strip_suffix(b"\r").unwrap_or(line);
        let Some(colon) = line.iter().position(|byte| *byte == b':') else {
            continue;
        };
        if eq_ignore_ascii_case(&line[..colon], b"content-length") {
            return parse_usize(trim_ascii(&line[colon + 1..]));
        }
    }
    0
}

fn eq_ignore_ascii_case(left: &[u8], right: &[u8]) -> bool {
    left.len() == right.len()
        && left
            .iter()
            .zip(right)
            .all(|(a, b)| a.to_ascii_lowercase() == *b)
}

fn trim_ascii(bytes: &[u8]) -> &[u8] {
    let mut start = 0;
    let mut end = bytes.len();
    while start < end && bytes[start].is_ascii_whitespace() {
        start += 1;
    }
    while end > start && bytes[end - 1].is_ascii_whitespace() {
        end -= 1;
    }
    &bytes[start..end]
}

fn parse_usize(bytes: &[u8]) -> usize {
    let mut value = 0_usize;
    for byte in bytes {
        if !byte.is_ascii_digit() {
            return 0;
        }
        value = value
            .saturating_mul(10)
            .saturating_add((byte - b'0') as usize);
    }
    value
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn finds_header_end() {
        assert_eq!(find_header_end(b"GET / HTTP/1.1\r\n\r\n"), Some(14));
    }

    #[test]
    fn parses_content_length() {
        assert_eq!(
            parse_content_length(b"POST / HTTP/1.1\r\nContent-Length: 42\r\n\r\n"),
            42
        );
        assert_eq!(
            parse_content_length(b"POST / HTTP/1.1\r\ncontent-length:  7\r\n"),
            7
        );
    }
}

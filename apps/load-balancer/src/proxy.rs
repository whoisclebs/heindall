use std::io::{Read, Write};
use std::net::{Shutdown, TcpStream};
use std::sync::Arc;
use std::time::Duration;

use crate::round_robin::RoundRobin;

const BUFFER_SIZE: usize = 32 * 1024;

pub struct Proxy {
    balancer: Arc<RoundRobin>,
}

impl Proxy {
    pub fn new(balancer: Arc<RoundRobin>) -> Self {
        Self { balancer }
    }

    pub fn handle(&self, mut client: TcpStream) -> std::io::Result<()> {
        client.set_read_timeout(Some(Duration::from_secs(2)))?;
        client.set_write_timeout(Some(Duration::from_secs(2)))?;

        let upstream_addr = self.balancer.next();
        let mut upstream = TcpStream::connect(upstream_addr)?;
        upstream.set_read_timeout(Some(Duration::from_secs(2)))?;
        upstream.set_write_timeout(Some(Duration::from_secs(2)))?;

        let mut request = Vec::with_capacity(BUFFER_SIZE);
        read_http_request(&mut client, &mut request)?;
        upstream.write_all(&request)?;
        upstream.shutdown(Shutdown::Write).ok();

        let mut buffer = [0_u8; BUFFER_SIZE];
        loop {
            let n = upstream.read(&mut buffer)?;
            if n == 0 {
                break;
            }
            client.write_all(&buffer[..n])?;
        }
        client.flush()?;
        Ok(())
    }
}

fn read_http_request(stream: &mut TcpStream, out: &mut Vec<u8>) -> std::io::Result<()> {
    let mut buffer = [0_u8; BUFFER_SIZE];
    let mut header_end = None;

    while header_end.is_none() {
        let n = stream.read(&mut buffer)?;
        if n == 0 {
            return Ok(());
        }
        out.extend_from_slice(&buffer[..n]);
        header_end = find_header_end(out);
    }

    if let Some(end) = header_end {
        let content_length = parse_content_length(&out[..end]);
        while out.len().saturating_sub(end + 4) < content_length {
            let n = stream.read(&mut buffer)?;
            if n == 0 {
                break;
            }
            out.extend_from_slice(&buffer[..n]);
        }
    }
    Ok(())
}

fn find_header_end(bytes: &[u8]) -> Option<usize> {
    bytes.windows(4).position(|window| window == b"\r\n\r\n")
}

fn parse_content_length(header: &[u8]) -> usize {
    let text = String::from_utf8_lossy(header);
    for line in text.lines() {
        let Some((name, value)) = line.split_once(':') else { continue };
        if name.eq_ignore_ascii_case("content-length") {
            return value.trim().parse().unwrap_or(0);
        }
    }
    0
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
        assert_eq!(parse_content_length(b"POST / HTTP/1.1\r\nContent-Length: 42"), 42);
    }
}

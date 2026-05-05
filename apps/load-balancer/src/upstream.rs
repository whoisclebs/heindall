use std::net::{SocketAddr, ToSocketAddrs};
use std::path::PathBuf;
use std::pin::Pin;
use std::sync::Mutex;
use std::task::{Context, Poll};
use std::time::Duration;

use tokio::io::{AsyncRead, AsyncWrite, ReadBuf};
use tokio::net::TcpStream;
#[cfg(unix)]
use tokio::net::UnixStream;
use tokio::time::timeout;

const CONNECT_TIMEOUT: Duration = Duration::from_millis(250);

#[derive(Debug)]
pub struct UpstreamPool {
    endpoint: UpstreamEndpoint,
    idle: Mutex<Vec<UpstreamStream>>,
    max_idle: usize,
}

#[derive(Debug)]
enum UpstreamEndpoint {
    Tcp(SocketAddr),
    Unix(PathBuf),
}

#[derive(Debug)]
pub enum UpstreamStream {
    Tcp(TcpStream),
    #[cfg(unix)]
    Unix(UnixStream),
}

impl UpstreamPool {
    pub fn new(addr: &str, max_idle: usize) -> std::io::Result<Self> {
        let endpoint = if let Some(path) = addr.strip_prefix("unix://") {
            UpstreamEndpoint::Unix(PathBuf::from(path))
        } else {
            let addr = addr.to_socket_addrs()?.next().ok_or_else(|| {
                std::io::Error::new(std::io::ErrorKind::InvalidInput, "empty upstream address")
            })?;
            UpstreamEndpoint::Tcp(addr)
        };
        Ok(Self {
            endpoint,
            idle: Mutex::new(Vec::with_capacity(max_idle)),
            max_idle,
        })
    }

    pub async fn acquire(&self) -> std::io::Result<UpstreamStream> {
        if let Some(stream) = self.idle.lock().expect("pool mutex").pop() {
            return Ok(stream);
        }

        match &self.endpoint {
            UpstreamEndpoint::Tcp(addr) => {
                let stream = timeout(CONNECT_TIMEOUT, TcpStream::connect(addr)).await??;
                stream.set_nodelay(true)?;
                Ok(UpstreamStream::Tcp(stream))
            }
            UpstreamEndpoint::Unix(path) => {
                #[cfg(unix)]
                {
                let stream = timeout(CONNECT_TIMEOUT, UnixStream::connect(path)).await??;
                Ok(UpstreamStream::Unix(stream))
                }
                #[cfg(not(unix))]
                {
                    let _ = path;
                    Err(std::io::Error::new(
                        std::io::ErrorKind::Unsupported,
                        "unix upstreams are not supported on this platform",
                    ))
                }
            }
        }
    }

    pub fn release(&self, stream: UpstreamStream) {
        let mut idle = self.idle.lock().expect("pool mutex");
        if idle.len() < self.max_idle {
            idle.push(stream);
        }
    }

    pub fn clear_idle(&self) {
        self.idle.lock().expect("pool mutex").clear();
    }
}

impl AsyncRead for UpstreamStream {
    fn poll_read(
        self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<std::io::Result<()>> {
        match self.get_mut() {
            UpstreamStream::Tcp(stream) => Pin::new(stream).poll_read(cx, buf),
            #[cfg(unix)]
            UpstreamStream::Unix(stream) => Pin::new(stream).poll_read(cx, buf),
        }
    }
}

impl AsyncWrite for UpstreamStream {
    fn poll_write(
        self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &[u8],
    ) -> Poll<std::io::Result<usize>> {
        match self.get_mut() {
            UpstreamStream::Tcp(stream) => Pin::new(stream).poll_write(cx, buf),
            #[cfg(unix)]
            UpstreamStream::Unix(stream) => Pin::new(stream).poll_write(cx, buf),
        }
    }

    fn poll_flush(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        match self.get_mut() {
            UpstreamStream::Tcp(stream) => Pin::new(stream).poll_flush(cx),
            #[cfg(unix)]
            UpstreamStream::Unix(stream) => Pin::new(stream).poll_flush(cx),
        }
    }

    fn poll_shutdown(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        match self.get_mut() {
            UpstreamStream::Tcp(stream) => Pin::new(stream).poll_shutdown(cx),
            #[cfg(unix)]
            UpstreamStream::Unix(stream) => Pin::new(stream).poll_shutdown(cx),
        }
    }
}

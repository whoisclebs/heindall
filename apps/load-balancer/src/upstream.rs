use std::net::{SocketAddr, ToSocketAddrs};
use std::sync::Mutex;
use std::time::Duration;

use tokio::net::TcpStream;
use tokio::time::timeout;

const CONNECT_TIMEOUT: Duration = Duration::from_millis(250);

#[derive(Debug)]
pub struct UpstreamPool {
    addr: SocketAddr,
    idle: Mutex<Vec<TcpStream>>,
    max_idle: usize,
}

impl UpstreamPool {
    pub fn new(addr: &str, max_idle: usize) -> std::io::Result<Self> {
        let addr = addr.to_socket_addrs()?.next().ok_or_else(|| {
            std::io::Error::new(std::io::ErrorKind::InvalidInput, "empty upstream address")
        })?;
        Ok(Self {
            addr,
            idle: Mutex::new(Vec::with_capacity(max_idle)),
            max_idle,
        })
    }

    pub async fn acquire(&self) -> std::io::Result<TcpStream> {
        if let Some(stream) = self.idle.lock().expect("pool mutex").pop() {
            return Ok(stream);
        }

        let stream = timeout(CONNECT_TIMEOUT, TcpStream::connect(self.addr)).await??;
        stream.set_nodelay(true)?;
        Ok(stream)
    }

    pub fn release(&self, stream: TcpStream) {
        let mut idle = self.idle.lock().expect("pool mutex");
        if idle.len() < self.max_idle {
            idle.push(stream);
        }
    }
}

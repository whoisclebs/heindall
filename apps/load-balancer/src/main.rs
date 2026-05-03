mod config;
mod proxy;
mod round_robin;

use std::net::TcpListener;
use std::sync::Arc;
use std::thread;

use config::Config;
use proxy::Proxy;
use round_robin::RoundRobin;

fn main() -> std::io::Result<()> {
    let config = Config::from_env();
    let listener = TcpListener::bind(&config.bind_addr)?;
    let balancer = Arc::new(RoundRobin::new(config.upstreams));

    for stream in listener.incoming() {
        let client = match stream {
            Ok(stream) => stream,
            Err(err) => {
                eprintln!("accept error: {err}");
                continue;
            }
        };
        let proxy = Proxy::new(Arc::clone(&balancer));
        thread::spawn(move || {
            if let Err(err) = proxy.handle(client) {
                eprintln!("proxy error: {err}");
            }
        });
    }

    Ok(())
}

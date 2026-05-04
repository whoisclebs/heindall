mod config;
mod proxy;
mod round_robin;
mod upstream;

use std::sync::Arc;

use config::Config;
use proxy::Proxy;
use upstream::UpstreamPool;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    tokio::runtime::Builder::new_current_thread()
        .enable_io()
        .enable_time()
        .build()?
        .block_on(async_main())
}

async fn async_main() -> Result<(), Box<dyn std::error::Error>> {
    let config = Config::from_env();
    let mut upstreams = Vec::with_capacity(config.upstreams.len());
    for upstream in &config.upstreams {
        upstreams.push(Arc::new(UpstreamPool::new(upstream, config.pool_size)?));
    }

    let proxy = Arc::new(Proxy::new(upstreams));
    proxy.serve(&config.bind_addr).await?;
    Ok(())
}

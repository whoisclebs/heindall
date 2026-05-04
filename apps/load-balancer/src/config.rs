use std::env;

#[derive(Debug, Clone)]
pub struct Config {
    pub bind_addr: String,
    pub upstreams: Vec<String>,
    pub pool_size: usize,
}

impl Config {
    pub fn from_env() -> Self {
        let bind_addr = env::var("BIND_ADDR").unwrap_or_else(|_| "0.0.0.0:9999".to_string());
        let upstreams = env::var("UPSTREAMS")
            .unwrap_or_else(|_| "api1:8080,api2:8080".to_string())
            .split(',')
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(ToOwned::to_owned)
            .collect::<Vec<_>>();

        if upstreams.is_empty() {
            panic!("UPSTREAMS must contain at least one upstream");
        }
        let pool_size = env::var("UPSTREAM_POOL_SIZE")
            .ok()
            .and_then(|value| value.parse().ok())
            .unwrap_or(32);

        Self {
            bind_addr,
            upstreams,
            pool_size,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_default_config() {
        env::remove_var("BIND_ADDR");
        env::remove_var("UPSTREAMS");
        let cfg = Config::from_env();
        assert_eq!(cfg.bind_addr, "0.0.0.0:9999");
        assert_eq!(cfg.upstreams, vec!["api1:8080", "api2:8080"]);
        assert_eq!(cfg.pool_size, 32);
    }
}

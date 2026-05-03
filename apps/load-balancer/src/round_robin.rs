use std::sync::atomic::{AtomicUsize, Ordering};

#[derive(Debug)]
pub struct RoundRobin {
    upstreams: Vec<String>,
    next: AtomicUsize,
}

impl RoundRobin {
    pub fn new(upstreams: Vec<String>) -> Self {
        assert!(!upstreams.is_empty(), "at least one upstream is required");
        Self { upstreams, next: AtomicUsize::new(0) }
    }

    pub fn next(&self) -> &str {
        let pos = self.next.fetch_add(1, Ordering::Relaxed) % self.upstreams.len();
        &self.upstreams[pos]
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rotates_upstreams() {
        let rr = RoundRobin::new(vec!["a".to_string(), "b".to_string()]);
        assert_eq!(rr.next(), "a");
        assert_eq!(rr.next(), "b");
        assert_eq!(rr.next(), "a");
    }
}

use std::sync::atomic::{AtomicUsize, Ordering};

#[derive(Debug)]
pub struct RoundRobin {
    upstream_count: usize,
    next: AtomicUsize,
}

impl RoundRobin {
    pub fn new(upstream_count: usize) -> Self {
        assert!(upstream_count > 0, "at least one upstream is required");
        Self {
            upstream_count,
            next: AtomicUsize::new(0),
        }
    }

    pub fn next(&self) -> usize {
        let value = self.next.fetch_add(1, Ordering::Relaxed);
        if self.upstream_count == 2 {
            value & 1
        } else {
            value % self.upstream_count
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rotates_two_upstreams() {
        let rr = RoundRobin::new(2);
        assert_eq!(rr.next(), 0);
        assert_eq!(rr.next(), 1);
        assert_eq!(rr.next(), 0);
    }
}

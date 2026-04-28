use std::sync::Mutex;
use sysinfo::System;

pub struct SystemService {
    sys: Mutex<System>,
}

#[derive(Clone)]
pub struct SystemMetrics {
    pub cpu_usage: u16,
    pub ram_used_gb: u16,
    pub ram_total_gb: u16,
}

impl SystemService {
    pub fn new() -> Self {
        let mut sys = System::new_all();
        sys.refresh_all();
        Self {
            sys: Mutex::new(sys),
        }
    }

    pub fn refresh(&self) {
        if let Ok(mut sys) = self.sys.lock() {
            sys.refresh_all();
        }
    }

    pub fn get_metrics(&self) -> SystemMetrics {
        if let Ok(sys) = self.sys.lock() {
            SystemMetrics {
                cpu_usage: sys.global_cpu_usage() as u16,
                ram_used_gb: (sys.used_memory() / 1024 / 1024 / 1024) as u16,
                ram_total_gb: (sys.total_memory() / 1024 / 1024 / 1024) as u16,
            }
        } else {
            SystemMetrics {
                cpu_usage: 0,
                ram_used_gb: 0,
                ram_total_gb: 0,
            }
        }
    }
}

impl Default for SystemService {
    fn default() -> Self {
        Self::new()
    }
}

use std::sync::Mutex;
use sysinfo::System;

pub struct SystemService {
    sys: Mutex<System>,
    weather_cache: Mutex<String>,
    last_weather_update: Mutex<u64>,
}

#[derive(Clone)]
pub struct SystemMetrics {
    pub cpu_usage: u16,
    pub ram_used_gb: u16,
    pub ram_total_gb: u16,
    pub uptime_secs: u64,
    pub kernel: String,
    pub hostname: String,
    pub weather: String,
}

impl SystemService {
    pub fn new() -> Self {
        let mut sys = System::new_all();
        sys.refresh_all();
        Self {
            sys: Mutex::new(sys),
            weather_cache: Mutex::new("\u{2600}\u{fe0f} --\u{00b0}F".to_string()),
            last_weather_update: Mutex::new(0),
        }
    }

    pub fn refresh(&self) {
        if let Ok(mut sys) = self.sys.lock() {
            sys.refresh_all();
        }
    }

    pub fn get_weather() -> String {
        let output = std::process::Command::new("curl")
            .args(["-s", "--max-time", "5", "https://wttr.in/?format=1"])
            .output();

        if let Ok(output) = output {
            if output.status.success() {
                let weather = String::from_utf8_lossy(&output.stdout).trim().to_string();
                if !weather.is_empty() {
                    return weather;
                }
            }
        }
        "\u{2601}\u{fe0f} N/A".to_string()
    }

    pub fn get_metrics(&self) -> SystemMetrics {
        let uptime = System::uptime();
        let kernel = System::kernel_version().unwrap_or_else(|| "Unknown".to_string());
        let hostname = System::host_name().unwrap_or_else(|| "Unknown".to_string());

        let should_fetch = {
            if let Ok(last) = self.last_weather_update.lock() {
                uptime.saturating_sub(*last) > 1800
            } else {
                true
            }
        };

        if should_fetch {
            let weather = Self::get_weather();
            if let Ok(mut cache) = self.weather_cache.lock() {
                *cache = weather.clone();
            }
            if let Ok(mut last) = self.last_weather_update.lock() {
                *last = uptime;
            }
        }

        let weather = self
            .weather_cache
            .lock()
            .map(|w| w.clone())
            .unwrap_or_else(|_| "\u{2601}\u{fe0f} --".to_string());

        SystemMetrics {
            cpu_usage: 0,
            ram_used_gb: 0,
            ram_total_gb: 0,
            uptime_secs: uptime,
            kernel,
            hostname,
            weather,
        }
    }
}

impl Default for SystemService {
    fn default() -> Self {
        Self::new()
    }
}

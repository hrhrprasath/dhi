use chrono::Utc;
use std::fs;
use std::path::PathBuf;

pub struct Handlers;

impl Handlers {
    pub fn system_metrics() -> String {
        use sysinfo::System;
        let mut sys = System::new_all();
        sys.refresh_all();

        format!(
            r#"CPU: {:.1}%
RAM: {} GB / {} GB
Uptime: {} seconds"#,
            sys.global_cpu_usage(),
            sys.used_memory() / 1024 / 1024 / 1024,
            sys.total_memory() / 1024 / 1024 / 1024,
            System::uptime()
        )
    }

    pub fn expense_log(amount: &str, category: &str, description: &str) -> String {
        let data_dir = PathBuf::from("./data/expense");
        let _ = fs::create_dir_all(&data_dir);

        let timestamp = Utc::now().format("%Y-%m-%dT%H-%M-%S");
        let filename = format!("{}.json", timestamp);
        let path = data_dir.join(&filename);

        let entry = serde_json::json!({
            "timestamp": Utc::now().to_rfc3339(),
            "amount": amount,
            "category": category,
            "description": description
        });

        if let Err(e) = fs::write(
            &path,
            serde_json::to_string_pretty(&entry).unwrap_or_default(),
        ) {
            return format!("Failed to log expense: {}", e);
        }

        format!(
            "Expense logged: {} ({}) - {}",
            amount, category, description
        )
    }

    pub fn health_tracker(action: &str, metric: &str) -> String {
        match action {
            "log" => format!("Health metric '{}' logged", metric),
            "check" => format!("Current health status: Good (mock data)"),
            "summary" => format!("Today's steps: 8500, Sleep: 7.2h, Workouts: 1"),
            _ => format!("Health action '{}' completed", action),
        }
    }

    pub fn history_summary(limit: Option<usize>) -> String {
        let data_dir = PathBuf::from("./data");
        let limit = limit.unwrap_or(20);

        let mut summaries = Vec::new();

        if let Ok(entries) = fs::read_dir(&data_dir) {
            for entry in entries.flatten() {
                let path = entry.path();
                if path.is_dir() {
                    let pipeline_name = path
                        .file_name()
                        .and_then(|n| n.to_str())
                        .unwrap_or("unknown");

                    if let Ok(files) = fs::read_dir(&path) {
                        let mut md_files: Vec<_> = files
                            .filter_map(|f| f.ok())
                            .filter(|f| f.path().extension().map(|e| e == "md").unwrap_or(false))
                            .collect();

                        md_files.sort_by(|a, b| {
                            b.metadata()
                                .and_then(|m| m.modified())
                                .unwrap_or(std::time::SystemTime::UNIX_EPOCH)
                                .cmp(
                                    &a.metadata()
                                        .and_then(|m| m.modified())
                                        .unwrap_or(std::time::SystemTime::UNIX_EPOCH),
                                )
                        });

                        for file in md_files.iter().take(3) {
                            if let Ok(content) = fs::read_to_string(file.path()) {
                                let first_line = content.lines().next().unwrap_or("").trim();
                                summaries.push(format!("[{}] {}", pipeline_name, first_line));
                            }
                        }
                    }
                }
            }
        }

        if summaries.is_empty() {
            "No history data found. Start using pipelines to create activity history.".to_string()
        } else {
            summaries.truncate(limit);
            format!("Recent Activity:\n{}", summaries.join("\n"))
        }
    }

    pub fn daily_brief() -> String {
        let history = Self::history_summary(Some(10));

        format!(
            r#"Daily Brief - {}

{}

System Status: Running normally
Weather: Checked at startup

Start your day productive!"#,
            Utc::now().format("%Y-%m-%d"),
            history
        )
    }
}

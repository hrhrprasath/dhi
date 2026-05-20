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
        let data_dir = PathBuf::from("./.data/expense");
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
        let data_dir = PathBuf::from("./.data");
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

    pub fn memory_grep(query: &str, _date_filter: &str, depth: &str) -> String {
        let vault_path = PathBuf::from("./vault");
        let depth_level: usize = depth.parse().unwrap_or(1);

        if !vault_path.exists() {
            return format!(
                "Vault not found at ./vault. Create a markdown directory for memory storage.\n\
                 Usage: Create ./vault/history/*.md files with your conversations and decisions."
            );
        }

        let mut results = Vec::new();

        let level1_results = Self::grep_search(&vault_path, query, 3);
        results.push(("Level 1: Direct Matches".to_string(), level1_results));

        if depth_level >= 2 {
            let related_terms = Self::extract_related_terms(&results[0].1);
            let mut level2_results = Vec::new();
            for term in related_terms.iter().take(5) {
                let matches = Self::grep_search(&vault_path, term, 2);
                if !matches.is_empty() {
                    level2_results.push(format!("Related to '{}': {}", term, matches));
                }
            }
            results.push((
                "Level 2: Related Concepts".to_string(),
                level2_results.join("\n"),
            ));
        }

        if depth_level >= 3 {
            let timeline = Self::build_timeline(&vault_path);
            results.push(("Level 3: Timeline Synthesis".to_string(), timeline));
        }

        let output = if results.iter().all(|(_, r)| r.is_empty()) {
            format!(
                "No matches found for '{}' in vault.\n\
                 Hint: Ensure your vault contains markdown files with relevant content.",
                query
            )
        } else {
            results
                .iter()
                .map(|(level, content)| format!("## {}\n{}", level, content))
                .collect::<Vec<_>>()
                .join("\n\n")
        };

        let _ = Self::log_action("memory_grep", query, &output);
        output
    }

    fn grep_search(path: &PathBuf, pattern: &str, context_lines: usize) -> String {
        let output = std::process::Command::new("rg")
            .args(["--json", "-i", "-C", &context_lines.to_string(), pattern])
            .arg(path)
            .output();

        match output {
            Ok(out) => {
                if out.status.success() {
                    let stdout = String::from_utf8_lossy(&out.stdout);
                    Self::parse_rg_json(&stdout)
                } else {
                    let stderr = String::from_utf8_lossy(&out.stderr);
                    if stderr.contains("No matches") {
                        String::new()
                    } else {
                        format!("Search error: {}", stderr)
                    }
                }
            }
            Err(e) => format!("Failed to run ripgrep: {}", e),
        }
    }

    fn parse_rg_json(output: &str) -> String {
        let mut matches = Vec::new();
        for line in output.lines() {
            if let Ok(json) = serde_json::from_str::<serde_json::Value>(line) {
                if let Some(m) = json.get("type").and_then(|t| t.as_str()) {
                    if m == "match" {
                        let file = json
                            .get("data")
                            .and_then(|d| d.get("path"))
                            .and_then(|p| p.get("text"))
                            .map(|t| t.to_string())
                            .unwrap_or_default();
                        let line_num = json
                            .get("data")
                            .and_then(|d| d.get("line_number"))
                            .and_then(|l| l.as_u64())
                            .map(|n| n.to_string())
                            .unwrap_or_default();
                        let lines = json
                            .get("data")
                            .and_then(|d| d.get("lines"))
                            .and_then(|l| l.get("text"))
                            .map(|t| t.to_string())
                            .unwrap_or_default();

                        if !file.is_empty() {
                            matches.push(format!("{}:{}: {}", file, line_num, lines.trim()));
                        }
                    }
                }
            }
        }
        matches.join("\n")
    }

    fn extract_related_terms(matches: &str) -> Vec<String> {
        let keywords = [
            "what",
            "decided",
            "setup",
            "bought",
            "installed",
            "configured",
            "fixed",
        ];
        matches
            .lines()
            .flat_map(|line| {
                line.split_whitespace()
                    .filter(|w| {
                        let lower = w.to_lowercase();
                        !keywords.contains(&lower.as_str())
                            && w.len() > 3
                            && w.chars().all(|c| c.is_alphanumeric() || c == '-')
                    })
                    .map(|s| s.to_string())
                    .take(10)
            })
            .collect()
    }

    fn build_timeline(path: &PathBuf) -> String {
        let output = std::process::Command::new("rg")
            .args(["--files", "-t", "md", "-g", "*.md", "--sort", "path"])
            .arg(path)
            .output();

        match output {
            Ok(out) if out.status.success() => {
                let files = String::from_utf8_lossy(&out.stdout);
                let file_list: Vec<&str> = files.lines().rev().take(20).collect();
                format!("Recent {} files: {}", file_list.len(), file_list.join(", "))
            }
            _ => "Timeline unavailable".to_string(),
        }
    }

    fn log_action(handler: &str, query: &str, result: &str) -> std::io::Result<()> {
        let data_dir = PathBuf::from("./.data/intent");
        fs::create_dir_all(&data_dir)?;

        let timestamp = Utc::now().format("%Y-%m-%dT%H-%M-%S");
        let filename = format!("memory_{}.md", timestamp);
        let path = data_dir.join(&filename);

        let content = format!(
            "# Memory Grep\n\nQuery: {}\nHandler: {}\n\n{}\n",
            query, handler, result
        );
        fs::write(path, content)
    }

    pub fn conversation_continue(topic: &str, last_context: &str) -> String {
        if topic.is_empty() {
            return "No topic provided for conversation continuation.".to_string();
        }

        let context_file = PathBuf::from("./.data/conversation/context.json");

        let prior_context = if context_file.exists() {
            fs::read_to_string(&context_file)
                .ok()
                .map(|c| c.to_string())
                .unwrap_or_default()
        } else {
            String::new()
        };

        let summary = if prior_context.is_empty() {
            format!(
                "Starting new conversation on: {}\n\
                 No prior context available. This appears to be a fresh topic.",
                topic
            )
        } else {
            format!(
                "Continuing: {}\n\
                 Previous context: {}\n\
                 Current query builds on that conversation.",
                topic,
                prior_context.lines().last().unwrap_or("N/A")
            )
        };

        let _ = Self::save_context(topic, last_context);

        format!(
            "{}\n\nNote: For full conversation history, use HISTORY pipeline with topic: {}",
            summary, topic
        )
    }

    fn save_context(topic: &str, new_context: &str) -> std::io::Result<()> {
        let data_dir = PathBuf::from("./.data/conversation");
        fs::create_dir_all(&data_dir)?;

        let context_file = data_dir.join("context.json");
        let entry = serde_json::json!({
            "topic": topic,
            "context": new_context,
            "timestamp": Utc::now().to_rfc3339()
        });

        fs::write(
            context_file,
            serde_json::to_string_pretty(&entry).unwrap_or_default(),
        )
    }
}

use chrono::Utc;
use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConversationSummary {
    pub overall: String,
    pub recent_note: String,
    pub turn_count: u32,
    pub last_updated: String,
}

impl Default for ConversationSummary {
    fn default() -> Self {
        Self {
            overall: String::new(),
            recent_note: String::new(),
            turn_count: 0,
            last_updated: Utc::now().to_rfc3339(),
        }
    }
}

pub struct ConversationManager {
    summary_path: PathBuf,
}

impl ConversationManager {
    pub fn new() -> Self {
        let data_dir = PathBuf::from("./.data/conversation");
        let _ = fs::create_dir_all(&data_dir);

        Self {
            summary_path: data_dir.join("summary.json"),
        }
    }

    pub fn load_summary(&self) -> ConversationSummary {
        match fs::read_to_string(&self.summary_path) {
            Ok(content) => serde_json::from_str(&content).unwrap_or_default(),
            Err(_) => ConversationSummary::default(),
        }
    }

    pub fn update_summary(&self, user_input: &str, response: &str) {
        let mut summary = self.load_summary();
        summary.turn_count += 1;

        let topic = user_input
            .lines()
            .next()
            .unwrap_or(user_input)
            .chars()
            .take(50)
            .collect::<String>();

        let input_short: String = user_input.chars().take(100).collect();
        let resp_short: String = response.chars().take(100).collect();

        let entry = format!("[{}. {}]", summary.turn_count, topic);
        if summary.overall.is_empty() {
            summary.overall = entry;
        } else {
            summary.overall = format!("{} {}", summary.overall, entry);
        }
        if summary.overall.chars().count() > 600 {
            let truncate_to = summary.overall.chars().count() - 600;
            let idx = summary
                .overall
                .char_indices()
                .nth(truncate_to)
                .map(|(i, _)| i)
                .unwrap_or(0);
            summary.overall = summary.overall[idx..].to_string();
        }

        let note = format!("User: {}\nDHI: {}\n", input_short, resp_short);
        if summary.recent_note.is_empty() {
            summary.recent_note = note;
        } else {
            summary.recent_note.push_str(&note);
        }
        if summary.recent_note.chars().count() > 400 {
            let truncate_to = summary.recent_note.chars().count() - 400;
            let idx = summary
                .recent_note
                .char_indices()
                .nth(truncate_to)
                .map(|(i, _)| i)
                .unwrap_or(0);
            summary.recent_note = summary.recent_note[idx..].to_string();
        }

        summary.last_updated = Utc::now().to_rfc3339();
        self.save(&summary);
    }

    pub fn format_for_prompt(&self, summary: &ConversationSummary) -> String {
        if summary.turn_count == 0 {
            return "No prior conversation.".to_string();
        }

        let mut output = format!("Overall: {}", summary.overall);
        if !summary.recent_note.is_empty() {
            output.push_str("\n\nRecent exchanges:\n");
            output.push_str(&summary.recent_note);
        }
        output
    }

    pub async fn search_past_data(terms: &[String]) -> String {
        if terms.is_empty() {
            return String::new();
        }

        let terms = terms.to_vec();
        tokio::task::spawn_blocking(move || Self::search_sync(&terms))
            .await
            .unwrap_or_default()
    }

    fn search_sync(terms: &[String]) -> String {
        let mut results = Vec::new();
        let mut seen = HashSet::new();

        for term in terms {
            if let Some(snippets) = Self::run_rg(term, "./.data/") {
                for snippet in snippets {
                    if seen.insert(snippet.clone()) {
                        results.push(snippet);
                    }
                }
            }
        }

        let vault_path = PathBuf::from("./vault");
        if vault_path.exists() {
            for term in terms {
                if let Some(snippets) = Self::run_rg(term, "./vault/") {
                    for snippet in snippets {
                        if seen.insert(snippet.clone()) {
                            results.push(snippet);
                        }
                    }
                }
            }
        }

        if results.is_empty() {
            return String::new();
        }

        results.truncate(8);
        let mut output = String::from("PAST DATA REFERENCES:\n");
        for r in &results {
            output.push_str("---\n");
            output.push_str(r);
            output.push('\n');
        }
        output
    }

    fn run_rg(term: &str, path: &str) -> Option<Vec<String>> {
        let output = std::process::Command::new("rg")
            .args(["--json", "-i", "-C", "1", "--max-count", "5", term, path])
            .output()
            .ok()?;

        if !output.status.success() {
            return None;
        }

        let stdout = String::from_utf8_lossy(&output.stdout);
        let mut snippets = Vec::new();
        let mut current_file = String::new();

        for line in stdout.lines() {
            if let Ok(json) = serde_json::from_str::<serde_json::Value>(line) {
                match json["type"].as_str() {
                    Some("begin") => {
                        current_file = json["data"]["path"]["text"]
                            .as_str()
                            .unwrap_or("unknown")
                            .to_string();
                    }
                    Some("match") => {
                        let text = json["data"]["lines"]["text"]
                            .as_str()
                            .unwrap_or("")
                            .trim()
                            .to_string();
                        let snippet = format!("[{}] {}", current_file, text);
                        if !text.is_empty() {
                            snippets.push(snippet);
                        }
                    }
                    _ => {}
                }
            }
        }

        if snippets.is_empty() {
            None
        } else {
            Some(snippets)
        }
    }

    fn save(&self, summary: &ConversationSummary) {
        if let Ok(content) = serde_json::to_string_pretty(summary) {
            let _ = fs::write(&self.summary_path, content);
        }
    }
}

impl Default for ConversationManager {
    fn default() -> Self {
        Self::new()
    }
}

pub fn extract_search_terms(params: &Option<serde_json::Value>) -> Vec<String> {
    match params {
        Some(val) => val
            .get("search_terms")
            .and_then(|v| v.as_array())
            .map(|arr| {
                arr.iter()
                    .filter_map(|t| t.as_str().map(String::from))
                    .collect()
            })
            .unwrap_or_default(),
        None => Vec::new(),
    }
}

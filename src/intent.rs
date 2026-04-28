use anyhow::{Result, anyhow};
use reqwest::Client;
use serde::{Deserialize, Serialize};

const OLLAMA_BASE_URL: &str = "http://localhost:11434";
const DEFAULT_MODEL: &str = "qwen2.5:1.5b";

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IntentRequest {
    pub model: String,
    pub prompt: String,
    pub stream: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IntentResponse {
    pub model: String,
    pub response: String,
    pub done: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParsedIntent {
    pub event: String,
    pub target: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub params: Option<serde_json::Value>,
}

pub struct IntentRouter {
    client: Client,
    model: String,
    system_prompt: String,
}

impl IntentRouter {
    pub fn new(model: Option<String>) -> Self {
        let model = model.unwrap_or_else(|| DEFAULT_MODEL.to_string());
        let system_prompt = r#"You are an intent router. Convert user input to strict JSON.
Output ONLY valid JSON, no explanations.
Example: {"event": "EXECUTE_PORTFOLIO_CHECK", "target": "NIFTY_50"}
Available events: EXECUTE_PORTFOLIO_CHECK, EXECUTE_RSS_FETCH, EXECUTE_MAIL_CHECK, 
EXECUTE_HEALTH_LOG, EXECUTE_DEEP_SEARCH, EXECUTE_SYSTEM_STATUS, QUIT_APP"#;

        Self {
            client: Client::new(),
            model,
            system_prompt: system_prompt.to_string(),
        }
    }

    pub async fn route(&self, user_input: &str) -> Result<ParsedIntent> {
        let prompt = format!("{}\n\nUser: {}", self.system_prompt, user_input);

        let request = IntentRequest {
            model: self.model.clone(),
            prompt,
            stream: false,
            format: Some(serde_json::json!({"type": "object"})),
        };

        let response = self.client
            .post(format!("{}/api/generate", OLLAMA_BASE_URL))
            .json(&request)
            .send()
            .await?;

        if !response.status().is_success() {
            return Err(anyhow!("Ollama request failed: {}", response.status()));
        }

        let ollama_resp: IntentResponse = response.json().await?;
        let parsed = self.parse_json_response(&ollama_resp.response)?;

        Ok(parsed)
    }

    fn parse_json_response(&self, response: &str) -> Result<ParsedIntent> {
        let trimmed = response.trim();
        let json_str = if trimmed.starts_with("```json") {
            trimmed.trim_start_matches("```json").trim_end_matches("```").trim()
        } else if trimmed.starts_with("```") {
            trimmed.trim_start_matches("```").trim_end_matches("```").trim()
        } else {
            trimmed
        };

        let parsed: serde_json::Value = serde_json::from_str(json_str)
            .map_err(|e| anyhow!("Failed to parse JSON: {} - Response: {}", e, json_str))?;

        Ok(ParsedIntent {
            event: parsed["event"]
                .as_str()
                .ok_or_else(|| anyhow!("Missing 'event' field"))?
                .to_string(),
            target: parsed["target"]
                .as_str()
                .ok_or_else(|| anyhow!("Missing 'target' field"))?
                .to_string(),
            params: parsed.get("params").cloned(),
        })
    }

    pub async fn health_check(&self) -> bool {
        match self.client.get(format!("{}/api/tags", OLLAMA_BASE_URL)).send().await {
            Ok(resp) => resp.status().is_success(),
            Err(_) => false,
        }
    }
}

impl Default for IntentRouter {
    fn default() -> Self {
        Self::new(None)
    }
}
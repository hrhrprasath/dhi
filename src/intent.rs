use crate::prompt::{classify_prompt, conversation_prompt};
use crate::services::Logger;
use anyhow::{Result, anyhow};
use chrono::Utc;
use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;
use std::sync::Arc;



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
    #[serde(skip_serializing_if = "Option::is_none")]
    pub params: Option<serde_json::Value>,
}

pub struct IntentRouter {
    client: Client,
    model: String,
    endpoint: String,
    logger: Arc<Logger>,
}

impl IntentRouter {
    pub fn new(model: String, endpoint: String, logger: Arc<Logger>) -> Self {
        logger.info("ROUTER", "Intent Router initialized");
        Self {
            client: Client::new(),
            model,
            endpoint,
            logger,
        }
    }

    pub async fn route(&self, user_input: &str, summary_text: &str) -> Result<ParsedIntent> {
        self.logger.info("ROUTER", format!("User input: {}", user_input));
        let prompt = classify_prompt(user_input, summary_text);

        self.save_to_file(&prompt);

        let request = IntentRequest {
            model: self.model.clone(),
            prompt,
            stream: false,
            format: Some(serde_json::json!({"type": "object"})),
        };

        let response = self
            .client
            .post(format!("{}/api/generate", self.endpoint))
            .json(&request)
            .send()
            .await?;

        if !response.status().is_success() {
            let err = format!("Ollama request failed: {}", response.status());
            self.logger.error("ROUTER", &err);
            return Err(anyhow!(err));
        }

        let ollama_resp: IntentResponse = response.json().await?;

        self.logger
            .debug("ROUTER", format!("Ollama raw response:\n{}", ollama_resp.response));

        let parsed = self.parse_json_response(&ollama_resp.response)?;

        self.logger
            .info("ROUTER", format!("Intent parsed: {:?}", parsed));

        Ok(parsed)
    }

    pub async fn chat(
        &self,
        user_input: &str,
        summary_text: &str,
        search_results: &str,
    ) -> Result<String> {
        self.logger
            .info("ROUTER", format!("Chat generation for: {}", user_input));
        let prompt = conversation_prompt(user_input, summary_text, search_results);

        let request = IntentRequest {
            model: self.model.clone(),
            prompt,
            stream: false,
            format: None,
        };

        let response = self
            .client
            .post(format!("{}/api/generate", self.endpoint))
            .json(&request)
            .send()
            .await?;

        if !response.status().is_success() {
            let err = format!("Ollama chat request failed: {}", response.status());
            self.logger.error("ROUTER", &err);
            return Err(anyhow!(err));
        }

        let ollama_resp: IntentResponse = response.json().await?;

        self.logger.debug(
            "ROUTER",
            format!("Chat response: {} chars", ollama_resp.response.len()),
        );

        Ok(ollama_resp.response.trim().to_string())
    }

    fn parse_json_response(&self, response: &str) -> Result<ParsedIntent> {
        let trimmed = response.trim();
        let json_str = if trimmed.starts_with("```json") {
            trimmed
                .trim_start_matches("```json")
                .trim_end_matches("```")
                .trim()
        } else if trimmed.starts_with("```") {
            trimmed
                .trim_start_matches("```")
                .trim_end_matches("```")
                .trim()
        } else {
            trimmed
        };

        let parsed: serde_json::Value = serde_json::from_str(json_str).map_err(|e| {
            let err = format!("Failed to parse JSON: {} - Response: {}", e, json_str);
            self.logger.error("ROUTER", &err);
            anyhow!(err)
        })?;

        Ok(ParsedIntent {
            event: parsed["event"]
                .as_str()
                .ok_or_else(|| anyhow!("Missing 'event' field"))?
                .to_string(),
            params: parsed.get("params").cloned(),
        })
    }

    pub async fn health_check(&self) -> bool {
        match self
            .client
            .get(format!("{}/api/tags", self.endpoint))
            .send()
            .await
        {
            Ok(resp) => resp.status().is_success(),
            Err(_) => false,
        }
    }

    fn save_to_file(&self, prompt: &str) {
        let data_dir = PathBuf::from("./.data/intent");
        let _ = fs::create_dir_all(&data_dir);

        let timestamp = Utc::now().format("%Y-%m-%dT%H-%M-%S").to_string();
        let filename = format!("{}.md", timestamp);
        let path = data_dir.join(&filename);

        let content = format!("# Intent Prompt\n\n{}", prompt);
        let _ = fs::write(&path, content);
    }
}

impl Default for IntentRouter {
    fn default() -> Self {
        Self::new(
            "qwen2.5:1.5b".to_string(),
            "http://localhost:11434".to_string(),
            Arc::new(Logger::new()),
        )
    }
}

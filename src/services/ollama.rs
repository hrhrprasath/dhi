use reqwest::Client;

const OLLAMA_BASE_URL: &str = "http://localhost:11434";

pub struct OllamaService {
    client: Client,
}

impl Clone for OllamaService {
    fn clone(&self) -> Self {
        Self {
            client: Client::new(),
        }
    }
}

#[derive(Clone)]
pub struct OllamaStatus {
    pub online: bool,
    pub status: String,
}

impl OllamaService {
    pub fn new() -> Self {
        Self {
            client: Client::new(),
        }
    }

    pub async fn health_check(&self) -> bool {
        match self.client.get(format!("{}/api/tags", OLLAMA_BASE_URL)).send().await {
            Ok(resp) => resp.status().is_success(),
            Err(_) => false,
        }
    }

    pub async fn check_status(&self) -> OllamaStatus {
        match self.client.get(format!("{}/api/tags", OLLAMA_BASE_URL)).send().await {
            Ok(resp) if resp.status().is_success() => OllamaStatus {
                online: true,
                status: "IDLE".to_string(),
            },
            Ok(resp) => OllamaStatus {
                online: false,
                status: format!("ERROR {}", resp.status().as_u16()),
            },
            Err(_) => OllamaStatus {
                online: false,
                status: "OFFLINE".to_string(),
            },
        }
    }
}

impl Default for OllamaService {
    fn default() -> Self {
        Self::new()
    }
}
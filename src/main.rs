mod intent;
mod services;
mod tui;

use anyhow::Result;
use crossterm::{
    event::{self, DisableMouseCapture, EnableMouseCapture, Event as CEvent, KeyCode},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::{backend::CrosstermBackend, Terminal};
use std::{
    io,
    sync::{Arc, Mutex as StdMutex},
    time::{Duration, Instant},
};
use tokio::sync::{broadcast, mpsc, Mutex as TokioMutex};

use intent::{IntentRouter, ParsedIntent};
use services::{Logger, OllamaService, SystemService};
use tui::components::command::{CommandState, FocusPanel};
use tui::components::logs::LogState;

pub enum SystemEvent {
    Input(crossterm::event::KeyEvent),
    Tick,
    SystemLog(String),
    PipelineUpdate { name: String, status: String },
    IntentResult { intent: ParsedIntent },
    IntentError(String),
    RouterOnline,
}

pub struct AppState {
    pub should_quit: bool,
    pub input_buffer: String,
    pub logs: Vec<String>,
    pub cpu_usage: u16,
    pub ram_usage_gb: u16,
    pub ram_total_gb: u16,
    pub nvidia_calls_per_min: u16,
    pub active_pipelines: Vec<(String, String)>,
    pub router_online: bool,
    pub router_status: String,
    pub synth_status: String,
    pub system_state: String,
    pub nvidia_api_status: String,
    pub animation_frame: usize,
    pub system_service: SystemService,
    pub ollama_service: OllamaService,
    pub logger: Arc<Logger>,
    pub log_state: Arc<StdMutex<LogState>>,
    pub cmd_state: Arc<StdMutex<CommandState>>,
    pub focused_panel: FocusPanel,
}

unsafe impl Send for AppState {}
unsafe impl Sync for AppState {}

impl AppState {
    #[allow(dead_code)]
    fn new() -> Self {
        Self::new_with_logger(
            Arc::new(Logger::new()),
            Arc::new(StdMutex::new(LogState::new())),
            Arc::new(StdMutex::new(CommandState::new())),
        )
    }

    fn new_with_logger(
        logger: Arc<Logger>,
        log_state: Arc<StdMutex<LogState>>,
        cmd_state: Arc<StdMutex<CommandState>>,
    ) -> Self {
        let system_service = SystemService::new();
        let metrics = system_service.get_metrics();

        logger.info("SYSTEM", "Aura Harness v1.0 booting...");

        Self {
            should_quit: false,
            input_buffer: String::new(),
            logs: vec!["[SYSTEM] Booting Aura Harness v1.0...".to_string()],
            cpu_usage: metrics.cpu_usage,
            ram_usage_gb: metrics.ram_used_gb,
            ram_total_gb: metrics.ram_total_gb,
            nvidia_calls_per_min: 12,
            active_pipelines: vec![
                ("RSS".to_string(), "Sleeping".to_string()),
                ("Portfolio".to_string(), "Executing".to_string()),
                ("Search".to_string(), "Idle".to_string()),
                ("Health".to_string(), "Idle".to_string()),
            ],
            router_online: false,
            router_status: "IDLE".to_string(),
            synth_status: "IDLE".to_string(),
            system_state: "SYNTHESIZING".to_string(),
            nvidia_api_status: "Online".to_string(),
            animation_frame: 0,
            system_service,
            ollama_service: OllamaService::new(),
            logger,
            log_state,
            cmd_state,
            focused_panel: FocusPanel::Logs,
        }
    }
}

#[tokio::main]
async fn main() -> Result<()> {
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let (tx, mut rx) = mpsc::channel::<SystemEvent>(100);

    let logger = Arc::new(Logger::new());
    let log_state = Arc::new(StdMutex::new(LogState::new()));
    let cmd_state = Arc::new(StdMutex::new(CommandState::new()));
    let log_state_clone = log_state.clone();
    let logger_clone = logger.clone();

    tokio::spawn(async move {
        let mut rx = logger_clone.subscribe();
        loop {
            match rx.recv().await {
                Ok(entry) => {
                    let mut log_state = log_state_clone.lock().unwrap();
                    log_state.add_log(entry);
                }
                Err(broadcast::error::RecvError::Lagged(n)) => {
                    let _ = logger_clone.warn("LOGGER", format!("Missed {} logs", n));
                }
                Err(broadcast::error::RecvError::Closed) => {
                    break;
                }
            }
        }
    });

    let app_state = Arc::new(TokioMutex::new(AppState::new_with_logger(logger, log_state, cmd_state)));

    let router = Arc::new(TokioMutex::new(IntentRouter::new(None)));

    let tick_rate = Duration::from_millis(250);
    let tx_input = tx.clone();
    std::thread::spawn(move || {
        let mut last_tick = Instant::now();
        loop {
            let timeout = tick_rate
                .checked_sub(last_tick.elapsed())
                .unwrap_or_else(|| Duration::from_secs(0));

            if crossterm::event::poll(timeout).expect("Poll failed") {
                if let CEvent::Key(key) = event::read().expect("Read failed") {
                    let _ = tx_input.blocking_send(SystemEvent::Input(key));
                }
            }
            if last_tick.elapsed() >= tick_rate {
                let _ = tx_input.blocking_send(SystemEvent::Tick);
                last_tick = Instant::now();
            }
        }
    });

    let tx_health = tx.clone();
    let router_clone = router.clone();
    let logger_for_health = app_state.lock().await.logger.clone();
    tokio::spawn(async move {
        logger_for_health.info("ROUTER", "Checking Ollama connection...");
        let r = router_clone.lock().await;
        let online = r.health_check().await;
        drop(r);
        if online {
            logger_for_health.info("ROUTER", "Ollama Intent Router ONLINE");
            let _ = tx_health.send(SystemEvent::RouterOnline).await;
        } else {
            logger_for_health.warn("ROUTER", "Ollama not available at localhost:11434");
            let _ = tx_health
                .send(SystemEvent::IntentError("Ollama not available at localhost:11434".to_string()))
                .await;
        }
    });

    let tx_pipeline = tx.clone();
    tokio::spawn(async move {
        tokio::time::sleep(Duration::from_secs(2)).await;
        let _ = tx_pipeline
            .send(SystemEvent::SystemLog("[SYNTH] NVIDIA NIM API ready (Phase 4)".to_string()))
            .await;
    });

    loop {
        let mut state = app_state.lock().await;
        terminal.draw(|f| tui::render(f, &state))?;

        if let Some(event) = rx.recv().await {
            match event {
                SystemEvent::Input(key) => match key.code {
                    KeyCode::Char('c') if key.modifiers.contains(event::KeyModifiers::CONTROL) => {
                        state.should_quit = true;
                    }
                    KeyCode::Tab => {
                        state.focused_panel = match state.focused_panel {
                            FocusPanel::Logs => FocusPanel::Command,
                            FocusPanel::Command => FocusPanel::Logs,
                        };
                    }
                    KeyCode::Up => {
                        match state.focused_panel {
                            FocusPanel::Logs => {
                                let mut log_state = state.log_state.lock().unwrap();
                                log_state.scroll = log_state.scroll.saturating_sub(1);
                            }
                            FocusPanel::Command => {
                                let mut cmd_state = state.cmd_state.lock().unwrap();
                                cmd_state.scroll = cmd_state.scroll.saturating_sub(1);
                            }
                        }
                    }
                    KeyCode::Down => {
                        let visible_lines = 10;
                        match state.focused_panel {
                            FocusPanel::Logs => {
                                let mut log_state = state.log_state.lock().unwrap();
                                let max_scroll = log_state.logs.len().saturating_sub(visible_lines) as u16;
                                log_state.scroll = (log_state.scroll + 1).min(max_scroll);
                            }
                            FocusPanel::Command => {
                                let mut cmd_state = state.cmd_state.lock().unwrap();
                                let max_scroll = cmd_state.commands.len().saturating_sub(visible_lines) as u16;
                                cmd_state.scroll = (cmd_state.scroll + 1).min(max_scroll);
                            }
                        }
                    }
                    KeyCode::Char(c) => state.input_buffer.push(c),
                    KeyCode::Backspace => {
                        state.input_buffer.pop();
                    }
                    KeyCode::Enter => {
                        let cmd = state.input_buffer.clone();
                        state.logs.push(format!("> {}", cmd));
                        state.cmd_state.lock().unwrap().add_command(cmd.clone());
                        state.input_buffer.clear();
                        if cmd == "quit" || cmd == "exit" {
                            state.should_quit = true;
                        } else if state.router_online {
                            let tx_intent = tx.clone();
                            let cmd_clone = cmd.clone();
                            let router_clone = router.clone();
                            tokio::spawn(async move {
                                let r = router_clone.lock().await;
                                match r.route(&cmd_clone).await {
                                    Ok(intent) => {
                                        let _ = tx_intent.send(SystemEvent::IntentResult { intent }).await;
                                    }
                                    Err(e) => {
                                        let _ = tx_intent
                                            .send(SystemEvent::IntentError(e.to_string()))
                                            .await;
                                    }
                                }
                            });
                        }
                    }
                    _ => {}
                },
                SystemEvent::SystemLog(msg) => state.logs.push(msg),
                SystemEvent::PipelineUpdate { name, status } => {
                    if let Some(p) = state.active_pipelines.iter_mut().find(|p| p.0 == name) {
                        p.1 = status;
                    }
                }
                SystemEvent::IntentResult { intent } => {
                    state
                        .logs
                        .push(format!("[ROUTER] Intent -> {} | {}", intent.target, intent.event));
                    state.router_online = true;
                    state.router_status = "PROCESSING".to_string();
                }
                SystemEvent::IntentError(err) => {
                    state
                        .logs
                        .push(format!("[ROUTER ERROR] {}", err));
                    state.router_online = false;
                    state.router_status = "OFFLINE".to_string();
                }
                SystemEvent::RouterOnline => {
                    state.router_online = true;
                    state.router_status = "IDLE".to_string();
                    state
                        .logs
                        .push("[ROUTER] Ollama Intent Router ONLINE".to_string());
                }
                SystemEvent::Tick => {
                    state.system_service.refresh();
                    let metrics = state.system_service.get_metrics();
                    state.cpu_usage = metrics.cpu_usage;
                    state.ram_usage_gb = metrics.ram_used_gb;
                    if state.router_status == "PROCESSING" {
                        state.router_status = "IDLE".to_string();
                    }
                    state.animation_frame = (state.animation_frame + 1) % 12;
                    if state.animation_frame == 0 {
                        state.logger.debug("TICK", "System tick");
                    }
                }
            }
        }
        if state.should_quit {
            break;
        }
    }

    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen, DisableMouseCapture)?;
    terminal.show_cursor()?;
    Ok(())
}
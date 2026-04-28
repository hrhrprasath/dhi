mod intent;

use anyhow::Result;
use crossterm::{
    event::{self, DisableMouseCapture, EnableMouseCapture, Event as CEvent, KeyCode},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::{
    backend::CrosstermBackend,
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    text::Line,
    widgets::{Block, Borders, Cell, Paragraph, Row, Table},
    Frame, Terminal,
};
use std::{
    io,
    sync::Arc,
    time::{Duration, Instant},
};
use tokio::sync::{mpsc, Mutex};

use intent::{IntentRouter, ParsedIntent};

const ANIMATION_FRAMES: &[&str; 12] = &[
    "        \n        \n        \n    .   \n        \n        \n        \n        ",
    "        \n        \n    ·   \n   ·.·  \n    ·   \n        \n        \n        ",
    "        \n    .   \n   · ·  \n  · o · \n   · ·  \n    .   \n        \n        ",
    "    ·   \n  ·   · \n ·     · \n·   o   ·\n ·     · \n  ·   · \n    ·   \n        ",
    "   · ·  \n  · · · \n · ·o· · \n· ·o·o· ·\n · ·o· · \n  · · · \n   · ·  \n        ",
    "  · · · \n · · · · \n· · o · ·\n· ·oOo· ·\n· · o · ·\n · · · · \n  · · · \n        ",
    " ·  ·  · \n·  · ·  ·\n · · · · \n· · O · ·\n · · · · \n·  · ·  ·\n ·  ·  · \n        ",
    "·   ·   ·\n ·  ·  · \n· · O · ·\n· O o O ·\n· · O · ·\n ·  ·  · \n·   ·   ·\n        ",
    " ·  ·  · \n·  · ·  ·\n · · · · \n· · O · ·\n · · · · \n·  · ·  ·\n ·  ·  · \n        ",
    "        \n  · · · \n · · · · \n · ·o· · \n · · · · \n  · · · \n        \n        ",
    "        \n        \n   · ·  \n  · · · \n   · ·  \n        \n        \n        ",
    "        \n        \n        \n    ·   \n        \n        \n        \n        ",
];

pub enum SystemEvent {
    Input(crossterm::event::KeyEvent),
    Tick,
    SystemLog(String),
    PipelineUpdate { name: String, status: String },
    IntentResult { intent: ParsedIntent },
    IntentError(String),
    RouterOnline,
}

struct AppState {
    should_quit: bool,
    input_buffer: String,
    logs: Vec<String>,
    cpu_usage: u16,
    ram_usage_gb: u16,
    nvidia_calls_per_min: u16,
    active_pipelines: Vec<(String, String)>,
    router_online: bool,
    router_status: String,
    synth_status: String,
    system_state: String,
    nvidia_api_status: String,
    animation_frame: usize,
}

impl AppState {
    fn new() -> Self {
        Self {
            should_quit: false,
            input_buffer: String::new(),
            logs: vec!["[SYSTEM] Booting Aura Harness v1.0...".to_string()],
            cpu_usage: 24,
            ram_usage_gb: 12,
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
    let app_state = Arc::new(Mutex::new(AppState::new()));
    let router = Arc::new(Mutex::new(IntentRouter::new(None)));

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
    tokio::spawn(async move {
        let r = router_clone.lock().await;
        let online = r.health_check().await;
        drop(r);
        if online {
            let _ = tx_health.send(SystemEvent::RouterOnline).await;
        } else {
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
        terminal.draw(|f| draw_ui(f, &state))?;

        if let Some(event) = rx.recv().await {
            match event {
                SystemEvent::Input(key) => match key.code {
                    KeyCode::Char('c') if key.modifiers.contains(event::KeyModifiers::CONTROL) => {
                        state.should_quit = true;
                    }
                    KeyCode::Char(c) => state.input_buffer.push(c),
                    KeyCode::Backspace => {
                        state.input_buffer.pop();
                    }
                    KeyCode::Enter => {
                        let cmd = state.input_buffer.clone();
                        state.logs.push(format!("> {}", cmd));
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
                    if state.router_status == "PROCESSING" {
                        state.router_status = "IDLE".to_string();
                    }
                    state.animation_frame = (state.animation_frame + 1) % 12;
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

fn draw_ui(f: &mut Frame, state: &AppState) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .margin(1)
        .constraints([Constraint::Percentage(99), Constraint::Percentage(1)].as_ref())
        .split(f.size());

    let main_split = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(12), Constraint::Percentage(88)])
        .split(chunks[0]);

    let left_chunk = main_split[0];
    let right_chunk = main_split[1];

    let left_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints(
            [
                Constraint::Percentage(40),
                Constraint::Percentage(30),
                Constraint::Percentage(10),
                Constraint::Length(10),
            ]
            .as_ref(),
        )
        .split(left_chunk);

    let metrics_text = vec![
        Line::from(format!("CPU: [||||||  ] {}%", state.cpu_usage)),
        Line::from(format!("RAM: [||||||||||] {}GB", state.ram_usage_gb)),
        Line::from(""),
        Line::from("NVIDIA API"),
        Line::from(format!("Calls: {} / min", state.nvidia_calls_per_min)),
        Line::from(""),
        Line::from(format!("[Ollama: {}]", state.router_status)),
    ];
    let metrics_block = Block::default()
        .borders(Borders::ALL)
        .title(" SYSTEM METRICS ");
    let metrics = Paragraph::new(metrics_text).block(metrics_block);
    f.render_widget(metrics, left_layout[0]);

    let header_cells = ["PIPELINE", "STATUS"]
        .iter()
        .map(|h| Cell::from(*h).style(Style::default().fg(Color::Blue)));
    let header = Row::new(header_cells)
        .style(Style::default().add_modifier(Modifier::BOLD))
        .height(1);
    let rows = state
        .active_pipelines
        .iter()
        .map(|p| Row::new(vec![p.0.clone(), p.1.clone()]));
    let table = Table::new(
        rows,
        [Constraint::Percentage(40), Constraint::Percentage(60)],
    )
    .header(header)
    .block(Block::default().borders(Borders::ALL).title(" ACTIVE QUEUES "));
    f.render_widget(table, left_layout[1]);

    let state_text = vec![Line::from(state.system_state.clone())];
    let state_block = Block::default().borders(Borders::ALL).title(" STATE ");
    let state_widget = Paragraph::new(state_text).block(state_block);
    f.render_widget(state_widget, left_layout[2]);

    let aura_area = left_layout[3];

    let aura_border = Block::default()
        .borders(Borders::ALL)
        .title(" AURA OS ")
        .title_style(Style::default().add_modifier(Modifier::BOLD));
    f.render_widget(aura_border, aura_area);

    let aura_inner = ratatui::layout::Rect::new(
        aura_area.x + 1,
        aura_area.y + 1,
        aura_area.right() - 1,
        aura_area.bottom() - 1,
    );

    let aura_split = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Length(14), Constraint::Min(8)])
        .split(aura_inner);

    let info_text = vec![
        Line::from("OS: Aura v1.0"),
        Line::from("Host: Mac M4"),
        Line::from("Kernel: "),
        Line::from("Up: 00:00:04"),
        Line::from("\u{2600}\u{fe0f} 72\u{00b0}F"),
    ];
    let info_widget = Paragraph::new(info_text).style(Style::default().fg(Color::White));
    f.render_widget(info_widget, aura_split[0]);

    let frame = ANIMATION_FRAMES[state.animation_frame];
    let anim_lines: Vec<Line> = frame.lines().map(Line::from).collect();
    let anim_widget = Paragraph::new(anim_lines).style(Style::default().fg(Color::Cyan));
    f.render_widget(anim_widget, aura_split[1]);

    let right_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints([Constraint::Percentage(30), Constraint::Percentage(67), Constraint::Length(3)].as_ref())
        .split(right_chunk);

    let logs: Vec<Line> = state.logs.iter().map(|l| Line::from(l.as_str())).collect();
    let log_view = Paragraph::new(logs)
        .block(Block::default().borders(Borders::ALL).title(" LOG "));
    f.render_widget(log_view, right_layout[0]);

    let command_center = Paragraph::new("Command center".to_string())
        .block(Block::default().borders(Borders::ALL).title(" COMMAND CENTER "));
    f.render_widget(command_center, right_layout[1]);

    let input = Paragraph::new(format!("> {}", state.input_buffer))
        .style(Style::default().fg(Color::Yellow))
        .block(Block::default().borders(Borders::ALL).title(" > "));
    f.render_widget(input, right_layout[2]);
}
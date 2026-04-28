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
    text::{Line, Span},
    widgets::{Block, Borders, Cell, Gauge, Paragraph, Row, Table},
    Terminal,
};
use std::{
    io,
    sync::Arc,
    time::{Duration, Instant},
};
use tokio::sync::{mpsc, Mutex};

pub enum SystemEvent {
    Input(crossterm::event::KeyEvent),
    Tick,
    SystemLog(String),
    PipelineUpdate { name: String, status: String },
}

struct AppState {
    should_quit: bool,
    input_buffer: String,
    logs: Vec<String>,
    cpu_usage: u16,
    active_pipelines: Vec<(String, String, String)>,
}

impl AppState {
    fn new() -> Self {
        Self {
            should_quit: false,
            input_buffer: String::new(),
            logs: vec!["[SYSTEM] Booting Aura Harness v1.0...".to_string()],
            cpu_usage: 12,
            active_pipelines: vec![
                ("Portfolio".to_string(), "Watching".to_string(), "12ms".to_string()),
                ("RSS_Scrape".to_string(), "Sleeping".to_string(), "--".to_string()),
                ("Health_Mon".to_string(), "Idle".to_string(), "--".to_string()),
            ],
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

    let tx_pipeline = tx.clone();
    tokio::spawn(async move {
        tokio::time::sleep(Duration::from_secs(2)).await;
        let _ = tx_pipeline.send(SystemEvent::SystemLog("[OLLAMA] Local 1B Intent Router verified.".to_string())).await;
        
        tokio::time::sleep(Duration::from_secs(3)).await;
        let _ = tx_pipeline.send(SystemEvent::PipelineUpdate {
            name: "Portfolio".to_string(),
            status: "ANALYZING INDEX".to_string(),
        }).await;
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
                    KeyCode::Backspace => { state.input_buffer.pop(); }
                    KeyCode::Enter => {
                        let cmd = state.input_buffer.clone();
                        state.logs.push(format!("> {}", cmd));
                        state.input_buffer.clear();
                        if cmd == "quit" || cmd == "exit" {
                            state.should_quit = true;
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
                SystemEvent::Tick => {}
            }
        }
        if state.should_quit { break; }
    }

    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen, DisableMouseCapture)?;
    terminal.show_cursor()?;
    Ok(())
}

fn draw_ui(f: &mut ratatui::Frame, state: &AppState) {
    let chunks = Layout::default().direction(Direction::Vertical).margin(1)
        .constraints([Constraint::Length(3), Constraint::Length(8), Constraint::Min(5), Constraint::Length(3)].as_ref())
        .split(f.size());

    let header_layout = Layout::default().direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(50), Constraint::Percentage(50)]).split(chunks[0]);

    let cpu_gauge = Gauge::default().block(Block::default().borders(Borders::ALL).title(" CPU USAGE "))
        .gauge_style(Style::default().fg(Color::Cyan).add_modifier(Modifier::ITALIC)).percent(state.cpu_usage);
    f.render_widget(cpu_gauge, header_layout[0]);

    let router_status = Paragraph::new(vec![
        Line::from(vec![Span::raw("ROUTER [Qwen 1.5B]  : "), Span::styled("IDLE", Style::default().fg(Color::Green))]),
        Line::from(vec![Span::raw("SYNTH  [NVIDIA NIM] : "), Span::styled("0 tk/s", Style::default().fg(Color::DarkGray))]),
    ]).block(Block::default().borders(Borders::ALL).title(" COGNITIVE LAYER "));
    f.render_widget(router_status, header_layout[1]);

    let header_cells = ["PIPELINE", "STATUS", "LATENCY"].iter().map(|h| Cell::from(*h).style(Style::default().fg(Color::Blue)));
    let header = Row::new(header_cells).style(Style::default().add_modifier(Modifier::BOLD)).height(1);
    let rows = state.active_pipelines.iter().map(|p| Row::new(vec![p.0.clone(), p.1.clone(), p.2.clone()]));
    let table = Table::new(rows, [Constraint::Percentage(30), Constraint::Percentage(50), Constraint::Percentage(20)])
        .header(header).block(Block::default().borders(Borders::ALL).title(" DAEMONS (Active Pipelines) "));
    f.render_widget(table, chunks[1]);

    let logs: Vec<Line> = state.logs.iter().map(|l| Line::from(l.as_str())).collect();
    let log_view = Paragraph::new(logs).block(Block::default().borders(Borders::ALL).title(" SYSTEM LOGS "));
    f.render_widget(log_view, chunks[2]);

    let input = Paragraph::new(format!("> {}", state.input_buffer)).style(Style::default().fg(Color::Yellow))
        .block(Block::default().borders(Borders::ALL).title(" COMMAND "));
    f.render_widget(input, chunks[3]);
}

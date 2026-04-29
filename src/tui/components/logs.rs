use crate::services::logger::{LogEntry, LogLevel};
use crate::tui::components::command::FocusPanel;
use crate::tui::palette::Palette;
use crate::AppState;
use ratatui::{
    layout::Rect,
    style::Style,
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph, Scrollbar, ScrollbarOrientation, ScrollbarState},
    Frame,
};

pub struct LogState {
    pub scroll: u16,
    pub logs: Vec<LogEntry>,
}

impl LogState {
    pub fn new() -> Self {
        Self {
            scroll: 0,
            logs: Vec::new(),
        }
    }

    pub fn add_log(&mut self, entry: LogEntry) {
        self.logs.push(entry);
    }

    pub fn auto_scroll(&mut self, visible_lines: usize) {
        if self.logs.len() > visible_lines {
            self.scroll = (self.logs.len() - visible_lines) as u16;
        } else {
            self.scroll = 0;
        }
    }
}

impl Default for LogState {
    fn default() -> Self {
        Self::new()
    }
}

fn format_log_entry(entry: &LogEntry) -> Line<'static> {
    let level_style = match entry.level {
        LogLevel::Debug => Style::default().fg(Palette::TEXT),
        LogLevel::Info => Style::default().fg(Palette::TEXT),
        LogLevel::Warn => Style::default().fg(Palette::WARNING),
        LogLevel::Error => Style::default().fg(Palette::ERROR),
    };

    let component_style = Style::default().fg(Palette::TEXT_ACCENT);
    let msg_style = Style::default().fg(Palette::TEXT_DIM);

    Line::from(vec![
        Span::raw("["),
        Span::styled(entry.component.clone(), component_style),
        Span::raw("]["),
        Span::styled(entry.level.as_str(), level_style),
        Span::raw("] "),
        Span::styled(entry.message.clone(), msg_style),
    ])
}

pub fn render(f: &mut Frame, state: &AppState, area: Rect, focused: FocusPanel) {
    let mut log_state = state.log_state.lock().unwrap();

    let is_focused = focused == FocusPanel::Logs;
    let focus_style = if is_focused {
        Style::default().fg(Palette::BORDER_FOCUSED)
    } else {
        Style::default().fg(Palette::BORDER_UNFOCUSED)
    };

    if log_state.logs.is_empty() {
        let empty = Paragraph::new("No logs yet...").block(
            Block::default()
                .borders(Borders::ALL)
                .border_style(focus_style)
                .title(" LOGS "),
        );
        f.render_widget(empty, area);
        return;
    }

    let visible_lines = area.height as usize - 2;
    if is_focused {
        log_state.auto_scroll(visible_lines);
    }

    let content_len = log_state.logs.len();
    let scroll_offset = log_state.scroll;

    let lines: Vec<Line> = log_state.logs.iter().map(format_log_entry).collect();

    let log_view = Paragraph::new(lines)
        .block(
            Block::default()
                .borders(Borders::ALL)
                .border_style(focus_style)
                .title(" LOGS "),
        )
        .scroll((scroll_offset, 0));

    f.render_widget(log_view, area);

    if is_focused {
        let mut scroll_state =
            ScrollbarState::new(scroll_offset as usize).content_length(content_len);
        let scrollbar = Scrollbar::new(ScrollbarOrientation::VerticalRight)
            .track_symbol(Some(" "))
            .style(Style::default().fg(Palette::BORDER_UNFOCUSED));
        f.render_stateful_widget(scrollbar, area, &mut scroll_state);
    }
}

#[allow(dead_code)]
pub fn handle_scroll(state: &AppState, delta: i32) {
    let mut log_state = state.log_state.lock().unwrap();
    let visible_lines = 10;
    let max_scroll = log_state.logs.len().saturating_sub(visible_lines) as i32;

    let new_scroll = (log_state.scroll as i32 + delta).clamp(0, max_scroll);
    log_state.scroll = new_scroll as u16;
}

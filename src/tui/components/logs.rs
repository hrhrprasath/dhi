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

    pub fn auto_scroll(&mut self, visible_lines: usize, width: usize) {
        let total_lines = self.total_wrapped_lines(width);
        if total_lines > visible_lines {
            self.scroll = (total_lines - visible_lines) as u16;
        } else {
            self.scroll = 0;
        }
    }

    pub fn total_wrapped_lines(&self, width: usize) -> usize {
        let prefix_len = 10;
        let wrap_width = width.saturating_sub(prefix_len + 1).max(10);
        self.logs
            .iter()
            .map(|e| wrap_text(&e.message, wrap_width).len().max(1))
            .sum()
    }
}

impl Default for LogState {
    fn default() -> Self {
        Self::new()
    }
}

fn wrap_text(text: &str, max_width: usize) -> Vec<String> {
    if max_width == 0 {
        return vec![text.to_string()];
    }

    let mut result = Vec::new();
    for line in text.lines() {
        if line.is_empty() {
            result.push(String::new());
            continue;
        }

        let mut current = String::new();
        for word in line.split_whitespace() {
            let word_len = word.chars().count();
            let current_len = current.chars().count();
            let next_len = if current.is_empty() {
                word_len
            } else {
                current_len + 1 + word_len
            };

            if next_len > max_width {
                if !current.is_empty() {
                    result.push(current);
                }
                current = word.to_string();
            } else {
                if current.is_empty() {
                    current = word.to_string();
                } else {
                    current.push(' ');
                    current.push_str(word);
                }
            }
        }
        if !current.is_empty() {
            result.push(current);
        }
    }
    result
}

fn format_log_entry(entry: &LogEntry, width: usize) -> Vec<Line<'static>> {
    let level_style = match entry.level {
        LogLevel::Debug => Style::default().fg(Palette::TEXT),
        LogLevel::Info => Style::default().fg(Palette::TEXT),
        LogLevel::Warn => Style::default().fg(Palette::WARNING),
        LogLevel::Error => Style::default().fg(Palette::ERROR),
    };

    let component_style = Style::default().fg(Palette::TEXT_ACCENT);
    let msg_style = Style::default().fg(Palette::TEXT_DIM);

    let prefix_len = entry.component.len() + entry.level.as_str().len() + 4;
    let wrap_width = width.saturating_sub(prefix_len + 1).max(10);

    let prefix = vec![
        Span::raw("["),
        Span::styled(entry.component.clone(), component_style),
        Span::raw("]["),
        Span::styled(entry.level.as_str(), level_style),
        Span::raw("] "),
    ];

    let wrapped = wrap_text(&entry.message, wrap_width);
    let mut result = Vec::new();

    if wrapped.is_empty() {
        result.push(Line::from(prefix.clone()));
        return result;
    }

    result.push(Line::from({
        let mut line = prefix.clone();
        line.push(Span::styled(wrapped[0].clone(), msg_style));
        line
    }));

    for line in wrapped.iter().skip(1) {
        result.push(Line::from(vec![Span::styled(line.clone(), msg_style)]));
    }

    result
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

    let width = area.width as usize;
    let visible_lines = area.height as usize - 2;
    if is_focused {
        log_state.auto_scroll(visible_lines, width);
    }

    let lines: Vec<Line> = log_state
        .logs
        .iter()
        .flat_map(|e| format_log_entry(e, width))
        .collect();
    let content_len = lines.len();
    let scroll_offset = log_state.scroll;

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
pub fn handle_scroll(state: &AppState, delta: i32, width: usize) {
    let mut log_state = state.log_state.lock().unwrap();
    let visible_lines = 10;
    let prefix_len = 10;
    let wrap_width = width.saturating_sub(prefix_len + 1).max(10);
    let total_lines: usize = log_state
        .logs
        .iter()
        .map(|e| wrap_text(&e.message, wrap_width).len().max(1))
        .sum();
    let max_scroll = total_lines.saturating_sub(visible_lines) as i32;

    let new_scroll = (log_state.scroll as i32 + delta).clamp(0, max_scroll);
    log_state.scroll = new_scroll as u16;
}

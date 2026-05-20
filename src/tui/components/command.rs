use crate::tui::palette::Palette;
use crate::AppState;
use ratatui::{
    layout::Rect,
    style::Style,
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph, Scrollbar, ScrollbarOrientation, ScrollbarState},
    Frame,
};

#[derive(Clone, Copy, PartialEq)]
pub enum FocusPanel {
    Logs,
    Command,
}

impl Default for FocusPanel {
    fn default() -> Self {
        FocusPanel::Logs
    }
}

#[derive(Clone)]
pub struct CommandState {
    pub scroll: u16,
    pub commands: Vec<String>,
}

impl CommandState {
    pub fn new() -> Self {
        Self {
            scroll: 0,
            commands: Vec::new(),
        }
    }

    pub fn add_command(&mut self, cmd: String) {
        self.commands.push(cmd);
    }
}

impl Default for CommandState {
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

pub fn render(f: &mut Frame, state: &AppState, area: Rect, focused: FocusPanel) {
    let mut cmd_state = state.cmd_state.lock().unwrap();
    let output_history = state.output_history.lock().unwrap();

    let is_focused = focused == FocusPanel::Command;
    let focus_style = if is_focused {
        Style::default().fg(Palette::BORDER_FOCUSED)
    } else {
        Style::default().fg(Palette::BORDER_UNFOCUSED)
    };

    let width = area.width as usize;
    let wrap_width = width.saturating_sub(10).max(20);

    let mut all_lines: Vec<Line> = Vec::new();

    for output in output_history.iter().rev().take(20) {
        let pipeline_char = output.pipeline.chars().next().unwrap_or('?');
        let prefix = format!("[{}] ", pipeline_char.to_uppercase());
        let wrapped = wrap_text(&output.summary, wrap_width.saturating_sub(prefix.len()));

        for (i, line) in wrapped.iter().enumerate() {
            if i == 0 {
                all_lines.push(Line::from(vec![
                    Span::styled(prefix.clone(), Style::default().fg(Palette::TEXT_ACCENT)),
                    Span::styled(line.clone(), Style::default().fg(Palette::TEXT_DIM)),
                ]));
            } else {
                all_lines.push(Line::from(vec![Span::styled(
                    line.clone(),
                    Style::default().fg(Palette::TEXT_DIM),
                )]));
            }
        }
    }

    if !cmd_state.commands.is_empty() {
        all_lines.push(Line::from(Span::styled(
            "───────────────────── Commands ────────────────────",
            Style::default().fg(Palette::TEXT_DIM),
        )));

        for (i, cmd) in cmd_state.commands.iter().enumerate().rev().take(10) {
            let prefix = format!("{:>4}> ", i + 1);
            let wrapped = wrap_text(cmd, wrap_width.saturating_sub(prefix.len()));

            for (j, line) in wrapped.iter().enumerate() {
                if j == 0 {
                    all_lines.push(Line::from(vec![
                        Span::styled(prefix.clone(), Style::default().fg(Palette::TEXT_ACCENT)),
                        Span::styled(line.clone(), Style::default().fg(Palette::TEXT_DIM)),
                    ]));
                } else {
                    all_lines.push(Line::from(vec![Span::styled(
                        line.clone(),
                        Style::default().fg(Palette::TEXT_DIM),
                    )]));
                }
            }
        }
    }

    if all_lines.is_empty() {
        let empty = Paragraph::new("No commands or outputs yet...").block(
            Block::default()
                .borders(Borders::ALL)
                .border_style(focus_style)
                .title(" COMMAND CENTER "),
        );
        f.render_widget(empty, area);
        return;
    }

    let visible_lines = area.height as usize - 2;
    if is_focused {
        if all_lines.len() > visible_lines {
            cmd_state.scroll = (all_lines.len() - visible_lines) as u16;
        } else {
            cmd_state.scroll = 0;
        }
    }

    let content_len = all_lines.len();
    let scroll_offset = cmd_state.scroll;

    let command_view = Paragraph::new(all_lines)
        .block(
            Block::default()
                .borders(Borders::ALL)
                .border_style(focus_style)
                .title(" COMMAND CENTER "),
        )
        .scroll((scroll_offset, 0));

    f.render_widget(command_view, area);

    if is_focused {
        let mut scroll_state =
            ScrollbarState::new(scroll_offset as usize).content_length(content_len);
        let scrollbar = Scrollbar::new(ScrollbarOrientation::VerticalRight)
            .track_symbol(Some(" "))
            .style(Style::default().fg(Palette::BORDER_UNFOCUSED));
        f.render_stateful_widget(scrollbar, area, &mut scroll_state);
    }
}

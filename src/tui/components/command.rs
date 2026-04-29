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

    pub fn auto_scroll(&mut self, visible_lines: usize) {
        if self.commands.len() > visible_lines {
            self.scroll = (self.commands.len() - visible_lines) as u16;
        } else {
            self.scroll = 0;
        }
    }
}

impl Default for CommandState {
    fn default() -> Self {
        Self::new()
    }
}

pub fn render(f: &mut Frame, state: &AppState, area: Rect, focused: FocusPanel) {
    let mut cmd_state = state.cmd_state.lock().unwrap();

    let is_focused = focused == FocusPanel::Command;
    let focus_style = if is_focused {
        Style::default().fg(Palette::BORDER_FOCUSED)
    } else {
        Style::default().fg(Palette::BORDER_UNFOCUSED)
    };

    if cmd_state.commands.is_empty() {
        let empty = Paragraph::new("No commands yet...").block(
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
        cmd_state.auto_scroll(visible_lines);
    }

    let content_len = cmd_state.commands.len();
    let scroll_offset = cmd_state.scroll;

    let lines: Vec<Line> = cmd_state
        .commands
        .iter()
        .enumerate()
        .map(|(i, cmd)| {
            Line::from(vec![
                Span::styled(
                    format!("{:>4}> ", i + 1),
                    Style::default().fg(Palette::TEXT_ACCENT),
                ),
                Span::styled(cmd.clone(), Style::default().fg(Palette::TEXT_DIM)),
            ])
        })
        .collect();

    let command_view = Paragraph::new(lines)
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

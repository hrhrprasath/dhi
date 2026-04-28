use crate::AppState;
use ratatui::{
    layout::{Constraint, Direction, Layout, Rect},
    style::{Color, Modifier, Style},
    text::Line,
    widgets::{Block, Borders, Paragraph},
    Frame,
};

use super::metrics::get_animation_frame;

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let aura_border = Block::default()
        .borders(Borders::ALL)
        .title(" AURA OS ")
        .title_style(Style::default().add_modifier(Modifier::BOLD));
    f.render_widget(aura_border, area);

    let aura_inner = Rect::new(area.x + 1, area.y + 1, area.right() - 1, area.bottom() - 1);

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

    let frame = get_animation_frame(state.animation_frame);
    let anim_lines: Vec<Line> = frame.lines().map(Line::from).collect();
    let anim_widget = Paragraph::new(anim_lines).style(Style::default().fg(Color::Cyan));
    f.render_widget(anim_widget, aura_split[1]);
}

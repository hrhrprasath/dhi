use crate::tui::palette::Palette;
use crate::AppState;
use ratatui::{
    layout::{Constraint, Rect},
    style::{Modifier, Style},
    widgets::{Block, Borders, Cell, Row, Table},
    Frame,
};

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
    let header_cells = ["PIPELINE", "STATUS"]
        .iter()
        .map(|h| Cell::from(*h).style(Style::default().fg(Palette::LABEL)));
    let header = Row::new(header_cells)
        .style(Style::default().add_modifier(Modifier::BOLD))
        .height(1);
    let rows = state.active_pipelines.iter().map(|p| {
        Row::new(vec![
            Cell::from(p.0.clone()).style(Style::default().fg(Palette::LABEL)),
            Cell::from(p.1.clone()).style(Style::default().fg(Palette::TEXT_ACCENT)),
        ])
    });
    let table = Table::new(
        rows,
        [Constraint::Percentage(40), Constraint::Percentage(60)],
    )
    .header(header)
    .block(
        Block::default()
            .borders(Borders::ALL)
            .title(" PIPELINE STATUS "),
    );
    f.render_widget(table, area);
}

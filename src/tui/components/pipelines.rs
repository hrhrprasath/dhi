use crate::AppState;
use ratatui::{
    layout::{Constraint, Rect},
    style::{Color, Modifier, Style},
    widgets::{Block, Borders, Cell, Row, Table},
    Frame,
};

pub fn render(f: &mut Frame, state: &AppState, area: Rect) {
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
    .block(
        Block::default()
            .borders(Borders::ALL)
            .title(" ACTIVE QUEUES "),
    );
    f.render_widget(table, area);
}

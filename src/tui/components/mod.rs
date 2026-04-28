pub mod aura;
pub mod command;
pub mod input;
pub mod logs;
pub mod metrics;
pub mod pipelines;
pub mod state;

use crate::AppState;
use ratatui::{
    layout::{Constraint, Direction, Layout},
    Frame,
};

pub fn render(f: &mut Frame, state: &AppState) {
    let size = f.size();

    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .margin(1)
        .constraints([Constraint::Percentage(99), Constraint::Percentage(1)].as_ref())
        .split(size);

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

    metrics::render(f, state, left_layout[0]);
    pipelines::render(f, state, left_layout[1]);
    state::render(f, state, left_layout[2]);
    aura::render(f, state, left_layout[3]);

    let right_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints(
            [
                Constraint::Percentage(30),
                Constraint::Percentage(67),
                Constraint::Length(3),
            ]
            .as_ref(),
        )
        .split(right_chunk);

    logs::render(f, state, right_layout[0]);
    command::render(f, state, right_layout[1]);
    input::render(f, state, right_layout[2]);
}

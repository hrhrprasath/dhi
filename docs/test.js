// /**
//  * AURA OS :: CONSISTENT ORB
//  * Fixed-width 7x4 character frames
//  */

// const frames = [
//   ["       ", "   .   ", "       ", "       "], // 1: Core
//   ["   '   ", "   .   ", "   '   ", "       "], // 2: Vertical
//   ["  .    ", " . . . ", "    .  ", "       "], // 3: Soften
//   ["  \\    ", "  .o.  ", "    /  ", "       "], // 4: Diagonals
//   ["  \\|   ", " --o-- ", "   |/  ", "       "], // 5: Expanding
//   ["  \\|/  ", " - o - ", "  /|\\  ", "       "], // 6: Full
//   ["  \\|/  ", " < O > ", "  /|\\  ", "       "], // 7: Peak (Glow)
//   ["  /|\\  ", " < O > ", "  \\|/  ", "       "], // 8: Peak (Rotate)
//   ["  /|\\  ", " - o - ", "  \\|/  ", "       "], // 9: Contracting
//   ["   |   ", "  .o.  ", "   |   ", "       "], // 10: Cooling
//   ["   .   ", "   .   ", "   .   ", "       "], // 11: Fading
//   ["       ", "   .   ", "       ", "       "]  // 12: Core
// ];

// let i = 0;

// // Hide cursor and clear screen
// process.stdout.write('\u001B[?25l\u001B[2J\u001B[H');

// const animation = setInterval(() => {
//   // Move cursor to top-left
//   process.stdout.write('\u001B[H');

//   frames[i].forEach(row => {
//     // We use a fixed-width print to ensure no shifting
//     process.stdout.write(`${row}\n`);
//   });

//   i = (i + 1) % frames.length;
// }, 120);

// // Cleanup on exit
// process.on('SIGINT', () => {
//   clearInterval(animation);
//   process.stdout.write('\u001B[2J\u001B[H\u001B[?25h');
//   process.exit();
// });

/**
 * AURA OS :: ZERO-JITTER ORB
 * This version uses a full-screen clear and a single-string draw
 * to ensure no dots shift between frames.
 */

const frames = [
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
  "        \n        \n        \n    ·   \n        \n        \n        \n        "
];

let i = 0;

// Hide cursor
process.stdout.write('\u001B[?25l');

const animation = setInterval(() => {
  // \u001B[2J clears screen, \u001B[H moves cursor to top left
  process.stdout.write('\u001B[2J\u001B[H');
  process.stdout.write(frames[i] + '\n');

  i = (i + 1) % frames.length;
}, 130);

process.on('SIGINT', () => {
  clearInterval(animation);
  process.stdout.write('\u001B[2J\u001B[H\u001B[?25h');
  process.exit();
});
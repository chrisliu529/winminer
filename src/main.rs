use image::{imageops, RgbaImage};
use rand::rngs::StdRng;
use rand::{Rng, SeedableRng};
use serde::Deserialize;
use std::collections::{BTreeMap, BTreeSet, HashMap};
use std::env;
use std::fs;
use std::time::Instant;

const UNKNOWN: i32 = 10;
const BOOM: i32 = 11;
const FLAG: i32 = 12;

#[derive(Clone, Deserialize)]
struct LevelConfig {
    row: usize,
    column: usize,
    mine: usize,
}

#[derive(Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct IsleConfig {
    max_mine: usize,
    max_size: usize,
}

#[derive(Deserialize)]
struct Config {
    #[serde(default)]
    strategies: Vec<String>,
    #[serde(default = "default_guess")]
    guess: String,
    #[serde(default = "default_accept")]
    accept: f64,
    #[serde(default)]
    verbose: bool,
    levels: Vec<LevelConfig>,
    isle: IsleConfig,
}

fn default_guess() -> String {
    "first".to_string()
}

fn default_accept() -> f64 {
    0.3
}

struct Options {
    config_file: String,
    cases_file: String,
    seed: i64,
    generate: bool,
    count: usize,
    level: usize,
    dump_text: bool,
    dump_prefix: String,
    dump_png: i32,
}

impl Default for Options {
    fn default() -> Self {
        Self {
            config_file: "winminer.toml".to_string(),
            cases_file: "cases.txt".to_string(),
            seed: 0,
            generate: false,
            count: 1,
            level: 1,
            dump_text: false,
            dump_prefix: String::new(),
            dump_png: 0,
        }
    }
}

fn main() {
    if let Err(error) = run() {
        eprintln!("{error}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let options = parse_args()?;
    if !(0..=3).contains(&options.dump_png) {
        return Err(format!("invalid dump mode: {}", options.dump_png));
    }
    let config_text = fs::read_to_string(&options.config_file)
        .map_err(|error| format!("read {}: {error}", options.config_file))?;
    let config: Config = toml::from_str(&config_text)
        .map_err(|error| format!("parse {}: {error}", options.config_file))?;
    if config.levels.is_empty() {
        return Err("configuration has no levels".to_string());
    }
    let images = if options.dump_png > 0 {
        Some(load_images()?)
    } else {
        None
    };
    if options.generate {
        if options.level == 0 || options.level > config.levels.len() {
            return Err(format!("invalid level: {}", options.level));
        }
        generate_cases(&config, &options);
    } else {
        run_benchmark(&config, &options, images.as_ref())?;
    }
    Ok(())
}

struct ImageAssets {
    tiles: HashMap<String, RgbaImage>,
}

fn load_images() -> Result<ImageAssets, String> {
    let names = [
        "0",
        "1",
        "2",
        "3",
        "4",
        "5",
        "6",
        "7",
        "8",
        "bomb_gray",
        "bomb_red",
        "flag",
        "unknown",
        "uncertain",
    ];
    let mut tiles = HashMap::new();
    for name in names {
        let filename = format!("image/{name}.png");
        let tile = image::open(&filename)
            .map_err(|error| format!("decode {filename}: {error}"))?
            .to_rgba8();
        tiles.insert(format!("{name}.png"), tile);
    }
    Ok(ImageAssets { tiles })
}

fn parse_args() -> Result<Options, String> {
    let mut options = Options::default();
    let args: Vec<String> = env::args().skip(1).collect();
    let mut index = 0;
    while index < args.len() {
        let arg = &args[index];
        if arg == "-h" || arg == "--help" {
            println!(
                "Usage: winminer-rust [-c config] [-f cases] [-s seed] [-gb] [-n count] [-lv level] [-dt] [-dt-file prefix] [-d mode]"
            );
            std::process::exit(0);
        }
        if arg == "-gb" {
            options.generate = true;
            index += 1;
            continue;
        }
        if arg == "-dt" {
            options.dump_text = true;
            index += 1;
            continue;
        }
        if arg == "-web" {
            return Err(
                "the Rust port implements the benchmark CLI, not the web server".to_string(),
            );
        }

        let (name, inline_value) = arg
            .split_once('=')
            .map(|(name, value)| (name, Some(value.to_string())))
            .unwrap_or((arg.as_str(), None));
        let value = if let Some(value) = inline_value {
            value
        } else {
            index += 1;
            args.get(index)
                .cloned()
                .ok_or_else(|| format!("missing value for {name}"))?
        };
        match name {
            "-c" => options.config_file = value,
            "-f" => options.cases_file = value,
            "-s" => {
                options.seed = value
                    .parse()
                    .map_err(|_| format!("invalid seed: {value}"))?
            }
            "-n" => {
                options.count = value
                    .parse()
                    .map_err(|_| format!("invalid case count: {value}"))?
            }
            "-lv" => {
                options.level = value
                    .parse()
                    .map_err(|_| format!("invalid level: {value}"))?
            }
            "-dt-file" => options.dump_prefix = value,
            "-d" => {
                options.dump_png = value
                    .parse()
                    .map_err(|_| format!("invalid dump mode: {value}"))?
            }
            _ => return Err(format!("unknown option: {arg}")),
        }
        index += 1;
    }
    Ok(options)
}

fn generate_cases(config: &Config, options: &Options) {
    let level = &config.levels[options.level - 1];
    let mut rng = StdRng::seed_from_u64(options.seed as u64);
    for _ in 0..options.count {
        let mut candidates: Vec<usize> = (0..level.row * level.column).collect();
        let mut mines = Vec::with_capacity(level.mine);
        while mines.len() < level.mine {
            let selected = rng.gen_range(0..candidates.len());
            if candidates[selected] == 0 {
                continue;
            }
            mines.push(candidates.remove(selected));
        }
        mines.sort_unstable();
        println!(
            "{}",
            mines
                .iter()
                .map(usize::to_string)
                .collect::<Vec<_>>()
                .join(",")
        );
    }
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum GameStatus {
    Playing,
    Lost,
    Won,
}

struct Board {
    tiles: Vec<i32>,
    row: usize,
    col: usize,
    status: GameStatus,
}

impl Board {
    fn new(level: &LevelConfig, mines: &[usize]) -> Self {
        let mut board = Self {
            tiles: vec![0; level.row * level.column],
            row: level.row,
            col: level.column,
            status: GameStatus::Playing,
        };
        for &mine in mines {
            board.tiles[mine] = -1;
        }
        for index in 0..board.tiles.len() {
            if board.tiles[index] < 0 {
                continue;
            }
            let adjacent_mines = board
                .neighbors(index)
                .iter()
                .filter(|&&neighbor| board.tiles[neighbor] < 0)
                .count() as i32;
            board.tiles[index] = adjacent_mines;
        }
        board
    }

    fn neighbors(&self, index: usize) -> Vec<usize> {
        let x = index % self.col;
        let y = index / self.col;
        let mut result = Vec::with_capacity(8);
        for ny in y.saturating_sub(1)..=(y + 1).min(self.row - 1) {
            for nx in x.saturating_sub(1)..=(x + 1).min(self.col - 1) {
                if nx != x || ny != y {
                    result.push(ny * self.col + nx);
                }
            }
        }
        result
    }

    fn render(&self) -> String {
        let mut result = String::new();
        for y in 0..self.row {
            for x in 0..self.col {
                result.push_str(&format!("{:2}", self.tiles[y * self.col + x]));
            }
            result.push('\n');
        }
        result
    }

    fn dump(&self, path: &str, images: &ImageAssets) -> Result<(), String> {
        let names: Vec<String> = self
            .tiles
            .iter()
            .map(|&tile| {
                if tile < 0 {
                    "bomb_gray.png".to_string()
                } else {
                    format!("{tile}.png")
                }
            })
            .collect();
        compose_png(self.row, self.col, &names, path, images)
    }
}

#[derive(Clone)]
struct PlayerTile {
    value: i32,
    revealed: bool,
}

#[derive(Clone)]
struct Constraint {
    cells: BTreeSet<usize>,
    mines: i32,
}

#[derive(Default, Clone)]
struct GuessStats {
    success: usize,
    failure: usize,
}

struct Player {
    tiles: Vec<PlayerTile>,
    view: Vec<Constraint>,
    sure: usize,
    guess: usize,
    row: usize,
    col: usize,
    mine: usize,
    gamename: String,
    guessed: String,
    guess_stats: BTreeMap<String, GuessStats>,
}

impl Player {
    fn new(level: usize, case_index: usize, config: &Config) -> Self {
        let level_config = &config.levels[level - 1];
        let mut guess_stats = BTreeMap::new();
        for name in ["first", "random", "corner", "min", "isle"] {
            guess_stats.insert(name.to_string(), GuessStats::default());
        }
        Self {
            tiles: vec![
                PlayerTile {
                    value: UNKNOWN,
                    revealed: false,
                };
                level_config.row * level_config.column
            ],
            view: Vec::new(),
            sure: 0,
            guess: 0,
            row: level_config.row,
            col: level_config.column,
            mine: level_config.mine,
            gamename: format!("{}-{}", level, case_index),
            guessed: "none".to_string(),
            guess_stats,
        }
    }

    fn is_unknown(&self, index: usize) -> bool {
        self.tiles[index].value == UNKNOWN
    }

    fn collect_unknown(&self) -> Vec<usize> {
        self.tiles
            .iter()
            .enumerate()
            .filter_map(|(index, tile)| (tile.value == UNKNOWN).then_some(index))
            .collect()
    }

    fn first_unknown(&self) -> Option<usize> {
        self.tiles.iter().position(|tile| tile.value == UNKNOWN)
    }

    fn corners(&self) -> Option<usize> {
        [
            0,
            self.row * self.col - 1,
            (self.row - 1) * self.col,
            self.col - 1,
        ]
        .into_iter()
        .find(|&index| self.is_unknown(index))
    }

    fn one_corner_guess(&self) -> Option<usize> {
        match self.guess % 4 {
            0 => self.first_unknown(),
            1 => (0..self.row)
                .rev()
                .flat_map(|y| (0..self.col).rev().map(move |x| y * self.col + x))
                .find(|&index| self.is_unknown(index)),
            2 => (0..self.row)
                .rev()
                .flat_map(|y| (0..self.col).map(move |x| y * self.col + x))
                .find(|&index| self.is_unknown(index)),
            _ => (0..self.row)
                .flat_map(|y| (0..self.col).rev().map(move |x| y * self.col + x))
                .find(|&index| self.is_unknown(index)),
        }
    }

    fn do_guess(&mut self, config: &Config, rng: &mut StdRng) -> Option<usize> {
        if let Some(corner) = self.corners() {
            self.guessed = "corner".to_string();
            return Some(corner);
        }
        self.guessed = config.guess.clone();
        match config.guess.as_str() {
            "first" => self.first_unknown(),
            "random" => {
                let unknown = self.collect_unknown();
                if unknown.is_empty() {
                    None
                } else {
                    Some(unknown[rng.gen_range(0..unknown.len())])
                }
            }
            "corner" => self.one_corner_guess(),
            "min" => self.min_guess(config),
            other => {
                self.guessed = other.to_string();
                self.first_unknown()
            }
        }
    }

    fn min_guess(&mut self, config: &Config) -> Option<usize> {
        let mut probabilities: HashMap<usize, f64> = HashMap::new();
        for constraint in &self.view {
            if constraint.mines <= 0 {
                continue;
            }
            let probability = constraint.mines as f64 / constraint.cells.len() as f64;
            for &cell in &constraint.cells {
                match probabilities.get_mut(&cell) {
                    Some(current) if *current < probability => *current = probability,
                    Some(_) => {}
                    None if probability < config.accept => {
                        probabilities.insert(cell, probability);
                    }
                    None => {}
                }
            }
        }
        if let Some((&cell, &probability)) =
            probabilities
                .iter()
                .min_by(|(cell_a, prob_a), (cell_b, prob_b)| {
                    prob_a.total_cmp(prob_b).then_with(|| cell_a.cmp(cell_b))
                })
        {
            verbose(config, &format!("Pmin = {probability}\n"));
            self.guessed = "min".to_string();
            Some(cell)
        } else {
            self.guessed = "corner".to_string();
            self.one_corner_guess()
        }
    }

    fn refresh_view(&mut self) -> Result<(), String> {
        let mut view = Vec::new();
        for index in 0..self.tiles.len() {
            let value = self.tiles[index].value;
            if self.tiles[index].revealed && (1..=8).contains(&value) {
                let (cells, remaining) = self.circle(index, value);
                if remaining < 0 {
                    return Err(format!(
                        "bad value: ({}, {}) = {remaining}",
                        index % self.col,
                        index / self.col
                    ));
                }
                if !cells.is_empty() {
                    view.push(Constraint {
                        cells,
                        mines: remaining,
                    });
                }
            }
        }
        self.view = view;
        Ok(())
    }

    fn circle(&self, index: usize, value: i32) -> (BTreeSet<usize>, i32) {
        let mut cells = BTreeSet::new();
        let mut remaining = value;
        for neighbor in self.neighbors(index) {
            match self.tiles[neighbor].value {
                UNKNOWN => {
                    cells.insert(neighbor);
                }
                FLAG => remaining -= 1,
                _ => {}
            }
        }
        (cells, remaining)
    }

    fn find_safe(&mut self, config: &Config) -> Result<Vec<usize>, String> {
        let mut state_changed = true;
        let mut need_refresh = true;
        let mut safe = Vec::new();
        while state_changed || need_refresh {
            state_changed = false;
            if need_refresh {
                self.refresh_view()?;
            }
            need_refresh = false;

            for constraint in self.view.clone() {
                if constraint.mines == 0 {
                    for cell in constraint.cells {
                        if !safe.contains(&cell) {
                            safe.push(cell);
                        }
                    }
                    return Ok(safe);
                }
                if constraint.mines as usize == constraint.cells.len() {
                    for cell in constraint.cells {
                        if self.tiles[cell].value != FLAG {
                            self.tiles[cell].value = FLAG;
                            self.mine -= 1;
                            if self.mine == 0 {
                                return Ok(self.collect_unknown());
                            }
                            need_refresh = true;
                        }
                    }
                }
            }

            if strategy_enabled(config, "diff") && !need_refresh {
                verbose(
                    config,
                    "searching view directly found no safe tiles, start searching diff\n",
                );
                let original = self.view.clone();
                let mut diffs: Vec<(Option<usize>, BTreeSet<usize>, i32)> = Vec::new();
                for outer in &original {
                    if outer.mines <= 0 {
                        continue;
                    }
                    for inner in &original {
                        if !outer.cells.is_superset(&inner.cells) || outer.cells == inner.cells {
                            continue;
                        }
                        let difference: BTreeSet<usize> =
                            outer.cells.difference(&inner.cells).copied().collect();
                        let mines = outer.mines - inner.mines;
                        let existing = original
                            .iter()
                            .position(|constraint| constraint.cells == difference);
                        if let Some(existing_index) = existing {
                            if original[existing_index].mines == mines {
                                continue;
                            }
                            if let Some((_, _, value)) = diffs
                                .iter_mut()
                                .find(|(index, _, _)| *index == Some(existing_index))
                            {
                                *value = mines;
                            } else {
                                diffs.push((Some(existing_index), difference, mines));
                            }
                        } else {
                            diffs.push((None, difference, mines));
                        }
                    }
                }
                for (existing_index, cells, mines) in diffs {
                    state_changed = true;
                    if let Some(index) = existing_index {
                        self.view[index].mines = mines;
                    } else {
                        self.view.push(Constraint { cells, mines });
                    }
                }
            }

            if strategy_enabled(config, "reduce") && !state_changed {
                verbose(
                    config,
                    "searching view diff made no state change, start searching reduce\n",
                );
                let original = self.view.clone();
                let mut reductions: Vec<Constraint> = Vec::new();
                for constraint in &original {
                    if constraint.mines > 1 {
                        for &cell in &constraint.cells {
                            let mut cells = constraint.cells.clone();
                            cells.remove(&cell);
                            reductions.push(Constraint {
                                cells,
                                mines: constraint.mines - 1,
                            });
                        }
                    }
                }
                for reduction in &reductions {
                    if let Some(parent) = original.iter().find(|candidate| {
                        candidate.cells.is_superset(&reduction.cells)
                            && candidate.cells != reduction.cells
                            && candidate.mines == reduction.mines
                    }) {
                        return Ok(parent.cells.difference(&reduction.cells).copied().collect());
                    }
                }
            }
        }

        if self.mine > 0 && strategy_enabled(config, "isle") {
            if self.mine < config.isle.max_mine {
                safe = self.find_isle(config);
            } else {
                verbose(
                    config,
                    &format!("remained {} mines, skip isle analysis\n", self.mine),
                );
            }
        }
        Ok(safe)
    }

    fn is_consistent(&self) -> bool {
        for (index, tile) in self.tiles.iter().enumerate() {
            if tile.revealed && (0..=8).contains(&tile.value) {
                let flags = self
                    .neighbors(index)
                    .iter()
                    .filter(|&&neighbor| self.tiles[neighbor].value == FLAG)
                    .count() as i32;
                if tile.value != flags {
                    return false;
                }
            }
        }
        true
    }

    fn neighbors(&self, index: usize) -> Vec<usize> {
        let x = index % self.col;
        let y = index / self.col;
        let mut result = Vec::with_capacity(8);
        for ny in y.saturating_sub(1)..=(y + 1).min(self.row - 1) {
            for nx in x.saturating_sub(1)..=(x + 1).min(self.col - 1) {
                if nx != x || ny != y {
                    result.push(ny * self.col + nx);
                }
            }
        }
        result
    }

    fn find_isle(&mut self, config: &Config) -> Vec<usize> {
        verbose(
            config,
            &format!("remained mines={}, locating the isle\n", self.mine),
        );
        let Some(start) = self.first_unknown() else {
            return Vec::new();
        };
        let isle = self.isle_at(start);
        if self.collect_unknown().len() != isle.len() {
            return Vec::new();
        }
        verbose(config, &format!("only one isle located: {:?}.\n", isle));
        if isle.len() > config.isle.max_size {
            verbose(config, "isle too large. giving up\n");
            return Vec::new();
        }
        verbose(config, &format!("Try mines ({}) simulations\n", self.mine));

        let mut solutions: Vec<BTreeSet<usize>> = Vec::new();
        let mut solution_mines: Vec<BTreeSet<usize>> = Vec::new();
        enumerate_combinations(
            isle.len(),
            self.mine,
            0,
            &mut Vec::with_capacity(self.mine),
            &mut |combination| {
                let mines: Vec<usize> = combination.iter().map(|&i| isle[i]).collect();
                for &mine in &mines {
                    self.tiles[mine].value = FLAG;
                }
                if self.is_consistent() {
                    solutions.push(self.collect_unknown().into_iter().collect());
                    solution_mines.push(mines.into_iter().collect());
                }
                for &mine in &isle {
                    if self.tiles[mine].value == FLAG {
                        self.tiles[mine].value = UNKNOWN;
                    }
                }
            },
        );

        if solutions.is_empty() {
            verbose(config, "isle: found no solution\n");
            return Vec::new();
        }
        if solutions.len() == 1 {
            self.mine = 0;
            for &mine in &solution_mines[0] {
                self.tiles[mine].value = FLAG;
            }
            return solutions[0].iter().copied().collect();
        }

        let mut frequencies: BTreeMap<usize, usize> =
            isle.iter().copied().map(|cell| (cell, 0)).collect();
        for solution in &solution_mines {
            for &mine in solution {
                *frequencies.entry(mine).or_default() += 1;
            }
        }
        let Some((&safest, &frequency)) = frequencies
            .iter()
            .min_by_key(|(cell, count)| (**count, **cell))
        else {
            return Vec::new();
        };
        if frequency == 0 {
            return vec![safest];
        }
        let probability = frequency as f64 / solutions.len() as f64;
        if probability < config.accept {
            verbose(config, &format!("PSmin = {probability}\n"));
            self.guessed = "isle".to_string();
            return vec![safest];
        }
        verbose(config, "isle: found multiple solutions\n");
        Vec::new()
    }

    fn isle_at(&self, start: usize) -> Vec<usize> {
        let mut result = Vec::new();
        let mut visited = vec![false; self.tiles.len()];
        let mut pending = vec![start];
        while let Some(index) = pending.pop() {
            if visited[index] {
                continue;
            }
            visited[index] = true;
            if self.tiles[index].value == UNKNOWN {
                result.push(index);
                let x = index % self.col;
                let y = index / self.col;
                if y + 1 < self.row {
                    pending.push(index + self.col);
                }
                if y > 0 {
                    pending.push(index - self.col);
                }
                if x + 1 < self.col {
                    pending.push(index + 1);
                }
                if x > 0 {
                    pending.push(index - 1);
                }
            }
        }
        result
    }

    fn click(&mut self, board: &mut Board, index: usize) {
        if index >= board.tiles.len() || self.tiles[index].revealed {
            return;
        }
        self.tiles[index].value = board.tiles[index];
        self.tiles[index].revealed = true;
        if board.tiles[index] < 0 {
            self.tiles[index].value = BOOM;
            board.status = GameStatus::Lost;
            return;
        }
        if board.tiles[index] == 0 {
            for neighbor in board.neighbors(index) {
                self.click(board, neighbor);
            }
        }
    }

    fn play(
        &mut self,
        board: &mut Board,
        config: &Config,
        rng: &mut StdRng,
        clicks: &mut usize,
        dump_mode: i32,
        images: Option<&ImageAssets>,
        png_counter: &mut usize,
    ) -> Result<GameStatus, String> {
        let mut step = 0;
        while board.status == GameStatus::Playing {
            if self.guessed != "none" {
                self.guess_stats
                    .entry(self.guessed.clone())
                    .or_default()
                    .success += 1;
                self.guessed = "none".to_string();
            }
            if self.sure == 0 {
                if self.is_unknown(0) {
                    self.dump_before_click(dump_mode, images, step, *png_counter)?;
                    if dump_mode == 3 {
                        *png_counter += 1;
                    }
                    self.click(board, 0);
                    *clicks += 1;
                    step += 1;
                }
                self.sure += 1;
                continue;
            }
            let safe = self.find_safe(config)?;
            if self.mine == 0 {
                board.status = GameStatus::Won;
                break;
            }
            for cell in &safe {
                if self.is_unknown(*cell) {
                    self.dump_before_click(dump_mode, images, step, *png_counter)?;
                    if dump_mode == 3 {
                        *png_counter += 1;
                    }
                    self.click(board, *cell);
                    *clicks += 1;
                    step += 1;
                }
                if self.guessed == "none" {
                    self.sure += 1;
                } else {
                    self.guess += 1;
                }
            }
            if safe.is_empty() && board.status == GameStatus::Playing {
                let Some(cell) = self.do_guess(config, rng) else {
                    return Err("bot has no available move".to_string());
                };
                self.dump_before_click(dump_mode, images, step, *png_counter)?;
                if dump_mode == 3 {
                    *png_counter += 1;
                }
                self.click(board, cell);
                *clicks += 1;
                step += 1;
                self.guess += 1;
            }
        }
        if dump_mode == 3 {
            self.dump(&format!("vs-{png_counter}.png"), images.unwrap())?;
            *png_counter += 1;
        }
        if board.status == GameStatus::Won && dump_mode == 2 {
            self.dump(&format!("f{}-{step}.png", self.gamename), images.unwrap())?;
        }
        if board.status == GameStatus::Lost {
            self.guess_stats
                .entry(self.guessed.clone())
                .or_default()
                .failure += 1;
        }
        Ok(board.status)
    }

    fn dump_before_click(
        &self,
        dump_mode: i32,
        images: Option<&ImageAssets>,
        step: usize,
        png_counter: usize,
    ) -> Result<(), String> {
        match dump_mode {
            2 => self.dump(&format!("f{}-{step}.png", self.gamename), images.unwrap()),
            3 => self.dump(&format!("vs-{png_counter}.png"), images.unwrap()),
            _ => Ok(()),
        }
    }

    fn dump(&self, path: &str, images: &ImageAssets) -> Result<(), String> {
        let names: Vec<String> = self
            .tiles
            .iter()
            .map(|tile| match tile.value {
                0..=8 => format!("{}.png", tile.value),
                BOOM => "bomb_red.png".to_string(),
                FLAG => "flag.png".to_string(),
                _ => "unknown.png".to_string(),
            })
            .collect();
        compose_png(self.row, self.col, &names, path, images)
    }
}

fn enumerate_combinations(
    n: usize,
    m: usize,
    start: usize,
    current: &mut Vec<usize>,
    callback: &mut impl FnMut(&[usize]),
) {
    if current.len() == m {
        callback(current);
        return;
    }
    let remaining = m - current.len();
    if n < remaining || start > n - remaining {
        return;
    }
    for value in start..=n - remaining {
        current.push(value);
        enumerate_combinations(n, m, value + 1, current, callback);
        current.pop();
    }
}

fn strategy_enabled(config: &Config, name: &str) -> bool {
    config.strategies.iter().any(|strategy| strategy == name)
}

fn compose_png(
    row: usize,
    col: usize,
    tile_names: &[String],
    path: &str,
    images: &ImageAssets,
) -> Result<(), String> {
    let mut canvas = RgbaImage::new((16 * col) as u32, (16 * row) as u32);
    for (index, name) in tile_names.iter().enumerate() {
        let tile = images
            .tiles
            .get(name)
            .ok_or_else(|| format!("missing image asset: {name}"))?;
        imageops::replace(
            &mut canvas,
            tile,
            (16 * (index % col)) as i64,
            (16 * (index / col)) as i64,
        );
    }
    canvas
        .save(path)
        .map_err(|error| format!("write {path}: {error}"))
}

fn verbose(config: &Config, message: &str) {
    if config.verbose {
        print!("{message}");
    }
}

fn run_benchmark(
    config: &Config,
    options: &Options,
    images: Option<&ImageAssets>,
) -> Result<(), String> {
    let text = fs::read_to_string(&options.cases_file)
        .map_err(|error| format!("read {}: {error}", options.cases_file))?;
    let lines: Vec<&str> = text.split('\n').collect();
    let mut wins = vec![0usize; config.levels.len()];
    let mut losses = vec![0usize; config.levels.len()];
    let mut guess_stats = BTreeMap::new();
    for name in ["first", "random", "corner", "min", "isle"] {
        guess_stats.insert(name.to_string(), GuessStats::default());
    }
    let mut total = 0usize;
    let mut sure = 0usize;
    let mut guess = 0usize;
    let mut total_clicks = 0usize;
    let mut click_count = 0usize;
    let mut successful_cases = Vec::new();
    let mut failed_cases = Vec::new();
    let mut rng = StdRng::seed_from_u64(options.seed as u64);
    let mut png_counter = 1;

    let started = Instant::now();
    for (line_index, line) in lines.iter().enumerate() {
        let parts: Vec<&str> = line.split(',').collect();
        if parts.len() <= 1 {
            continue;
        }
        total += 1;
        let mines: Vec<usize> = parts
            .iter()
            .map(|part| {
                part.parse::<usize>().map_err(|error| {
                    format!("invalid mine position on line {}: {error}", line_index + 1)
                })
            })
            .collect::<Result<_, _>>()?;
        let level_index = config
            .levels
            .iter()
            .position(|level| level.mine == mines.len())
            .ok_or_else(|| format!("no configured level has {} mines", mines.len()))?;
        let mut board = Board::new(&config.levels[level_index], &mines);
        if options.dump_png == 2 {
            board.dump(&format!("{line_index}.png"), images.unwrap())?;
        }
        if config.verbose {
            print!("{}\n", board.render());
        }
        let mut player = Player::new(level_index + 1, line_index, config);
        let status = player.play(
            &mut board,
            config,
            &mut rng,
            &mut click_count,
            options.dump_png,
            images,
            &mut png_counter,
        )?;
        if status == GameStatus::Won {
            wins[level_index] += 1;
            if options.dump_text {
                successful_cases.push(line.to_string());
            }
        } else {
            if options.dump_png >= 1 && player.sure >= 10 {
                player.dump(&format!("f{}.png", player.gamename), images.unwrap())?;
            }
            losses[level_index] += 1;
            if options.dump_text {
                failed_cases.push(line.to_string());
            }
        }
        sure += player.sure;
        guess += player.guess;
        total_clicks += player.sure + player.guess;
        for (name, stats) in player.guess_stats {
            let total_stats = guess_stats.entry(name).or_default();
            total_stats.success += stats.success;
            total_stats.failure += stats.failure;
        }
    }
    let elapsed_ms = started.elapsed().as_millis();
    let win_count: usize = wins.iter().sum();
    let loss_count: usize = losses.iter().sum();
    let denominator = total as f64;
    let win_rate = win_count as f64 / denominator;
    let loss_rate = loss_count as f64 / denominator;
    println!(
        "win: {:.2} ({}, {}, {}), lose: {:.2}",
        win_rate,
        wins.first().copied().unwrap_or_default(),
        wins.get(1).copied().unwrap_or_default(),
        wins.get(2).copied().unwrap_or_default(),
        loss_rate
    );
    let sure_ratio = sure as f64 / total_clicks as f64;
    let guess_ratio = guess as f64 / total_clicks as f64;
    let clicks_per_second = if elapsed_ms == 0 {
        0
    } else {
        (click_count as u128 * 1000 / elapsed_ms) as usize
    };
    println!(
        "sure: {}({:.2}), guess: {}({:.2}), {} clicks in {} ms ({} clicks/s)",
        sure, sure_ratio, guess, guess_ratio, click_count, elapsed_ms, clicks_per_second
    );
    for (name, stats) in guess_stats {
        println!("{}: success = {}", name, stats.success);
        println!("{}: failure = {}", name, stats.failure);
    }

    if options.dump_text {
        let success_path = format!("{}_success.txt", options.dump_prefix);
        let failed_path = format!("{}_failed.txt", options.dump_prefix);
        fs::write(&success_path, lines_with_newlines(&successful_cases))
            .map_err(|error| format!("write {success_path}: {error}"))?;
        fs::write(&failed_path, lines_with_newlines(&failed_cases))
            .map_err(|error| format!("write {failed_path}: {error}"))?;
    }
    Ok(())
}

fn lines_with_newlines(lines: &[String]) -> String {
    if lines.is_empty() {
        String::new()
    } else {
        format!("{}\n", lines.join("\n"))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn board_calculates_adjacent_mine_counts() {
        let board = Board::new(
            &LevelConfig {
                row: 3,
                column: 3,
                mine: 1,
            },
            &[0],
        );
        assert_eq!(board.tiles, vec![-1, 1, 0, 1, 1, 0, 0, 0, 0]);
    }

    #[test]
    fn enumerates_combinations_without_materializing_them() {
        let mut combinations = Vec::new();
        enumerate_combinations(4, 2, 0, &mut Vec::new(), &mut |combination| {
            combinations.push(combination.to_vec());
        });
        assert_eq!(
            combinations,
            vec![
                vec![0, 1],
                vec![0, 2],
                vec![0, 3],
                vec![1, 2],
                vec![1, 3],
                vec![2, 3]
            ]
        );
    }
}

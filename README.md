# Music Shuffler

A terminal app that picks random tracks from your local music library.

## What it does

Every music player's shuffle is broken. They all end up playing the same 200 tracks out of your 40,000. I got tired of it, so I wrote this instead.

Music Shuffler scans your configured directories, picks 10 random files (filtering out anything matching your ignore patterns), and shows them in a numbered list. Press a number to play it. Press `r` to get a fresh batch. There's a little animated EQ visualiser while tracks are playing, because why not.

macOS only for now -- it shells out to `afplay`.

## Prerequisites

- Go 1.21+
- macOS (uses `afplay` for audio playback)

## Getting started

```bash
git clone git@github.com:ohnotnow/music-shuffler.git
cd music-shuffler
go build -o music-shuffler .
```

Before running it, create a config file. The app looks for `ms.yaml` in the current directory first, then `~/.config/ms/config.yaml`.

```yaml
music_dirs:
  - /path/to/your/music/

ignore_patterns:
  - audiobook
  - spoken.*word

track_count: 10
```

`music_dirs` accepts multiple paths, so you can point it at several drives or folders. `ignore_patterns` are regular expressions matched case-insensitively against the full file path. `track_count` controls how many tracks to show (max 10).

Then run it:

```bash
./music-shuffler
```

## Usage

| Key | Action |
|-----|--------|
| `0-9` | Play that track |
| `s` | Stop playback |
| `r` | Shuffle a new set of tracks |
| `p` / `space` | Toggle between relative and full file paths |
| `q` | Quit (prints the last playing track's full path to the terminal) |

When you quit while a track is playing, the full path is printed to stdout so you can easily copy it.

## Contributing

Fork it, hack on it, run `go build .` to check it compiles. No tests yet (I know, I know). PRs welcome.

## Licence

[MIT](LICENSE)

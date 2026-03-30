# Music Shuffler

A terminal app that picks random tracks from your local music library.

## What it does

Every music player's shuffle is broken. They all end up playing the same 200 tracks out of your 40,000. I got tired of it, so I wrote this instead.

Music Shuffler scans your configured directories, picks 10 random files (filtering out anything matching your ignore patterns), and shows them in a numbered list. Press a number to play it. Press `r` to get a fresh batch. There's a little animated EQ visualiser while tracks are playing, because why not.

Works on macOS and Linux. It shells out to a configurable player command (`afplay` by default, but you can set it to `mpv`, `aplay`, or whatever you like).

## Prerequisites

- Go 1.21+ (or grab a pre-built binary from the [releases page](https://github.com/ohnotnow/music-shuffler/releases))
- An audio player command (`afplay` on macOS, `mpv` or `aplay` on Linux)

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

# Command used to play audio files (default: afplay)
# Examples: mpv, aplay, ffplay -nodisp -autoexit
player: afplay
```

`music_dirs` accepts multiple paths, so you can point it at several drives or folders. `ignore_patterns` are regular expressions matched case-insensitively against the full file path. `track_count` controls how many tracks to show (max 10). `player` is the command used to play files, defaulting to `afplay`. Linux users will want to change this to `mpv`, `aplay`, or similar.

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

# imgcrush

A quick, non-hacky command-line tool for compressing JPEG and PNG
images. One binary, no dependencies, pure Go.

## Project overview

imgcrush is a simple, honest image compression CLI. Not as good as
ImageOptim at raw compression, but it just works: single binary, no
external dependencies, no shelling out, proper Unix behavior.

Safe by default, honest about tradeoffs, and a proper Unix citizen
(flags, exit codes, stdout/stderr, composable with pipes and scripts).

## Session protocol

- Every session begins with /start and ends with /close.

## Key references

- **dev/RESEARCH.md** — comprehensive background document. Includes:
  - Section 0: detailed profile of ImageOptim (architecture, all 13
    bundled tools, GUI behavior, CLI limitations, what it gets right
    and wrong) plus jpegoptim CLI as a design model
  - Sections 1-4: Go image library research (format detection, JPEG
    and PNG compression in pure Go, library options, tradeoffs)
  - Section 5: metadata preservation research (library landscape,
    JPEG/PNG metadata formats, field-by-field DROP/KEEP/UPDATE
    analysis, raw byte-level splicing design, Six Hats analysis)
  - Design implications summary
  - **Consult before making any implementation decisions** about
    encoding, metadata, library choices, or CLI design.
- **dev/SPEC.md** — feature spec with vision, MVP scope, and backlog.
- **dev/AAR.md** — after action review / continuous improvement log.
- **dev/publication.taskpaper** — release and distribution plan.

## Install and run

```bash
go install .
imgcrush [flags] <files...>
```

## Test

```bash
go test ./...
```

## Design principles

- **Pure Go only.** No C dependencies, no CGo, no shelling out to
  external tools. If pure Go can't do it yet, we wait or write it.
- **Safe by default.** Backup before overwrite, skip-if-minimal-gain,
  dry-run support. Casual use on real files must be safe.
- **Proper Unix citizen.** Flags, exit codes, stderr for errors, stdout
  for output. Works in scripts, CI, Makefiles, SSH sessions.
- **Honest about tradeoffs.** imgcrush JPEG savings come from lossy
  re-encoding, not smarter compression. ImageOptim (without JPEGmini)
  does lossless optimization — a fundamentally different operation.
  Don't frame imgcrush results as "better than ImageOptim." See
  dev/RESEARCH.md section 4 benchmark for details.
- **jpegoptim as CLI design model.** Its flag design (threshold, dry-run,
  output dir, granular strip flags, stdin/stdout) is what good Unix
  tools look like. See dev/RESEARCH.md section 0.

## Code style

- Go standard formatting (gofmt)
- Keep it simple: one main package until complexity warrants splitting
- Error messages go to stderr, normal output to stdout

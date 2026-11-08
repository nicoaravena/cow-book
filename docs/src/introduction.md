# Introduction

This book goes with the project in [the repository](https://gitlab.com/nicoaravena/cow-book): a
multi-tenant coworking booking platform, built mainly to **learn Go**. The project
[README](https://gitlab.com/nicoaravena/cow-book/-/blob/main/README.md) describes the problem, and
the [Roadmap](ROADMAP.md) lays out the steps.

## How to read it

- **Concepts** has one page per roadmap step. Each page explains the Go ideas and domain ideas that
  step introduced, with short excerpts from the code as it was at that step. The topic index lists
  every topic and the steps where it shows up.
- **Reference** describes the code as it is now: what each type does, its rules, its errors and how
  the pieces fit together. Exact signatures live in the doc comments; read them with `go doc`:

  ```sh
  go doc ./cowbook            # package overview
  go doc ./cowbook Booking    # one type and its methods
  go doc -all ./cowbook       # everything
  ```

## Building the site

The site is built with [mdBook](https://rust-lang.github.io/mdBook/), version 0.5.4 at the time of
writing. Install it once:

```sh
cargo install mdbook --locked
```

Then, from the repository root:

```sh
make docs         # build into book/
make docs-serve   # serve with live reload and open a browser
```

`book/` is build output and is not committed.

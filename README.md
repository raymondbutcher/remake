# Remake

Remake is a very simple tool for running Make commands automatically.
Use `remake [target]` instead of the usual `make [target]` and it will
automatically run the command each time changes are made.

Gone are the days where you have to switch to a terminal and
then press Ctrl+C, Up, Enter after making changes!

## What does Remake do?

In basically runs `make [target]` and then watches for changes.
There are 2 scenarios:

1. The make command exits quickly

    Next time you make changes, Remake will run the command again.

1. The make command is a long-running process

    Next time you make changes, Remake will kill the process
    and then run the command again.

## Requirements

* OSX/Linux/Unix?
* Make
    * Tested with:
      * GNU Make 4.1 (Ubuntu 18.04)
      * GNU Make 4.2.1 (Debian 10 WSL2)
* Go
    * Only required because no binary releases have been created yet.
    * Tested with go1.14.4 linux/amd64

## Installation

It is not worth releasing binaries at this early stage.
So, for now, it must be built and installed using Go.

    $ go get github.com/raymondbutcher/remake

## Usage

Using Remake is like using Make, except that it keeps building the target
whenever something has changed. If you would normally run `make` then you
would run `remake`. Instead of `make dist` it would be `remake dist`.

Remake has been designed to require no configuration and no command line
arguments. Still though, there are some options if the default behavior
does not suit.

### Help

Usage: `remake -h` or `remake -help`

Displays the available command line options.

### Check interval

Usage: `remake -check=2s [target]`

This controls how often Remake checks for changes. The default interval is `2s`.

### Grace period

Usage: `remake -grace=10s [target]`

Scenario: a make command builds a HTTP development server and then runs it in
the foreground. When it starts up, it can take a second or two to build the
binary, minify CSS and JS files, etc, to satisfy its dependencies. It then
runs the HTTP server and serves requests.

It would not be good to restart the command while it is starting up and
building everything as instructed. So there is a grace period of 10 seconds,
configurable with the `-grace` command line option.

During the grace period, Remake will regularly check to see if
everything is up to date yet. As soon as it is, normal monitoring
begins. If the grace period is exceeded, and the command is still
running, then it will be restarted.

### Ready signal

Usage: `remake -ready`

To be more precise during the grace period, `remake -ready` can be run
from within a make command to let it know that the build phase of the
command has finished. Remake will immediately leave the grace period
and start monitoring for changes as usual.

This is not particularly useful, nor very noticeable; it just allows Remake
to be slightly more responsive to changes.

Because Remake won't necessarily be installed everywhere, it makes sense to
have the `remake -ready` command fail silently when used in a Makefile.

For example:

```makefile
# Shortcut for running "remake -ready" if available.
READY=remake -ready 2>/dev/null || true

http: bin/myapp
    @$(READY)
    @echo Starting HTTP server...
    bin/myapp -http

bin/myapp: $(wildcard src/myapp/*.go)
    @echo Building my app...
    go install myapp
```

Note: The ready signal has no effect when Remake is running multiple targets,
because it cannot tell which command sent the signal. The grace period will
work as normal.

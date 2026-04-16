# bash-cli

A reference unpack of **bash**'s command-line interface — every invocation flag, every pseudo-flag, how positional parameters flow in, which startup files run for which invocation shape, plus pointers to authoritative resources.

Primary source: `man bash` (sections `SYNOPSIS`, `OPTIONS`, `ARGUMENTS`, `INVOCATION`, `EXIT STATUS`, `FILES`, `RESTRICTED SHELL`) and `bash --help`. Cross-checked against the upstream GNU Bash Reference Manual.

---

## 1. Synopsis

```
bash [long-option] [option] ...
bash [long-option] [option] script-file [argument ...]
bash [long-option] [option] -c command_string [command_name [argument ...]]
```

Three invocation shapes cover essentially all usage:

| Shape                    | What bash does                                                                 |
|--------------------------|--------------------------------------------------------------------------------|
| `bash`                   | Interactive shell (if stdin/stderr are TTYs) or reads commands from stdin      |
| `bash script.sh arg ...` | Execute `script.sh`; `$0=script.sh`, `$1..` set from remaining args            |
| `bash -c 'cmd' NAME a b` | Run `cmd`; `$0=NAME`, `$1=a`, `$2=b`. If `NAME` omitted, `$0` is bash itself.  |

`bash -s ...` and `bash ...` with no filename both read from stdin.

---

## 2. Single-character (short) options

These are consumed by bash *before* script execution. A `--` ends option parsing; `-` is a synonym for `--`.

| Flag  | Meaning                                                                         |
|-------|---------------------------------------------------------------------------------|
| `-c`  | Read commands from the next argument (a string), not from a file or stdin.      |
| `-i`  | Force interactive mode even if stdin/stderr aren't TTYs.                        |
| `-l`  | Act as a login shell. Equivalent to `--login`, or to invocation as `-bash`.     |
| `-r`  | Restricted shell. See [§7 Restricted shell](#7-restricted-shell-rbash).         |
| `-s`  | Read commands from stdin *even when* positional args are provided.              |
| `-D`  | Dump all `$"…"` (locale-translated) strings found in the script, then exit.     |
| `-O shopt_name` | Enable a `shopt` option at invocation. Repeatable.                      |
| `+O shopt_name` | Disable a `shopt` option at invocation. Repeatable.                     |
| `--`  | End of options; remaining args are script + positional parameters.              |
| `-`   | Same as `--`.                                                                   |

Short options documented under `set` (see §5) are **also** accepted at invocation. The most commonly-passed-at-invocation ones:

| Flag  | `set -o` name   | Effect                                                             |
|-------|-----------------|--------------------------------------------------------------------|
| `-e`  | `errexit`       | Exit on any simple command with non-zero status.                   |
| `-u`  | `nounset`       | Error on referencing unset variables.                              |
| `-x`  | `xtrace`        | Print each command (after expansion) to stderr before running it.  |
| `-v`  | `verbose`       | Print each input line to stderr as it's read.                      |
| `-n`  | `noexec`        | Parse but don't execute. Used for syntax-checking scripts.         |
| `-f`  | `noglob`        | Disable pathname expansion (globbing).                             |
| `-a`  | `allexport`     | Auto-export every variable that's modified or created.             |
| `-b`  | `notify`        | Report background job completion immediately, not at next prompt.  |
| `-h`  | `hashall`       | Cache command locations (PATH hash) as they're looked up.          |
| `-k`  | `keyword`       | Put assignment args into the command's environment, not just leading ones. |
| `-m`  | `monitor`       | Job control (on by default for interactive shells).                |
| `-p`  | `privileged`    | Auto-on when real != effective uid/gid. Skips env-driven startup.  |
| `-t`  | `onecmd`        | Exit after one command.                                            |
| `-B`  | `braceexpand`   | Brace expansion (on by default).                                   |
| `-C`  | `noclobber`     | `>` won't overwrite existing files; use `>\|` to force.            |
| `-H`  | `histexpand`    | `!`-style history expansion (on for interactive).                  |
| `-P`  | `physical`      | `cd` doesn't follow symlinks; use physical paths.                  |
| `-E`  | `errtrace`      | Let shell functions inherit the `ERR` trap.                        |
| `-T`  | `functrace`     | Let shell functions inherit the `DEBUG` and `RETURN` traps.        |

Invert any of these by using `+` instead of `-` (e.g. `set +e`). `-o NAME` / `+o NAME` work at invocation too, for options that don't have a short-letter form (`pipefail`, `posix`, `emacs`, `vi`, `ignoreeof`, `interactive-comments`, etc.).

Very common invocation pragma: **`#!/bin/bash` + `set -euo pipefail`** — fail on errors, fail on unset vars, propagate failures through pipes.

---

## 3. Long (GNU-style) options

These must appear **before** the single-character options.

| Option                  | Effect                                                                                    |
|-------------------------|-------------------------------------------------------------------------------------------|
| `--debug`               | Bash debug profile (present on some distros' 3.x; on modern bash see `--debugger`).       |
| `--debugger`            | Turn on `shopt extdebug` + `set -o functrace` before the shell starts.                    |
| `--dump-po-strings`     | Like `-D`, but emit in GNU gettext `.po` format.                                          |
| `--dump-strings`        | Same as `-D`.                                                                             |
| `--help`                | Print the same usage bash emits on bad args; exit 0.                                      |
| `--init-file FILE`      | Read `FILE` instead of `~/.bashrc` for interactive startup.                               |
| `--rcfile FILE`         | Alias for `--init-file`.                                                                  |
| `--login`               | Same as `-l`.                                                                             |
| `--noediting`           | Disable readline line editing in interactive mode.                                        |
| `--noprofile`           | Skip `/etc/profile`, `~/.bash_profile`, `~/.bash_login`, `~/.profile`.                    |
| `--norc`                | Skip `~/.bashrc` in interactive shells. On by default when invoked as `sh`.               |
| `--posix`               | Enter POSIX mode — bash behavior matches POSIX wherever it differs from bash default.     |
| `--protected`           | (3.x historical) Protected-mode sandbox flag. Not present in most modern builds.          |
| `--restricted`          | Same as `-r`.                                                                             |
| `--verbose`             | Same as `-v`.                                                                             |
| `--version`             | Print version + copyright; exit 0.                                                        |
| `--wordexp`             | Internal; used by `wordexp(3)` implementations. Not for end-user invocation.              |

---

## 4. Positional parameters & `$0`

How arguments populate `$0`, `$1`, `$#`, `"$@"`:

```
bash                                 # $0=bash                  $#=0
bash script.sh a b                   # $0=script.sh  $1=a $2=b   $#=2
bash -c 'echo $0 $1' myname a b      # $0=myname     $1=a $2=b   $#=2
bash -c 'echo $0 $1' '' a b          # $0=''         $1=a $2=b   $#=2   (common idiom)
bash -s a b <<< 'echo $1 $2'         # $0=bash       $1=a $2=b   $#=2
```

`--` terminates bash's option parsing, not the script's:

```
bash -- -x --help   # runs a file literally named "-x", passing "--help"
```

---

## 5. Shell-option interface (`set` / `shopt`)

Bash has **two** parallel option systems accessible after startup. Most can also be set at invocation.

### `set` options

- Short form: `set -x` / `set +x`
- Long form: `set -o xtrace` / `set +o xtrace`
- Introspect: `set -o` prints current state; `$-` contains the active short-flag letters

Full list: `bash -c 'help set'` or see [§2](#2-single-character-short-options).

### `shopt` options

Bash-specific toggles (POSIX has no equivalent). Richer catalog than `set`; includes behavioral tweaks like:

| shopt              | What it does                                                               |
|--------------------|----------------------------------------------------------------------------|
| `nocaseglob`       | Case-insensitive pathname expansion.                                       |
| `nocasematch`      | Case-insensitive `case` / `[[ ... = ... ]]` matching.                      |
| `extglob`          | Enable `@(…)` `+(…)` `?(…)` `*(…)` `!(…)` patterns.                        |
| `globstar`         | `**` matches files across directory levels.                                |
| `nullglob`         | Non-matching globs expand to zero args, not to themselves.                 |
| `failglob`         | Non-matching globs are a hard error.                                       |
| `dotglob`          | `*` includes dotfiles.                                                     |
| `inherit_errexit` | Command substitution inherits `set -e` (fixes long-standing footgun).      |
| `lastpipe`         | Last stage of a pipeline runs in the current shell (can write variables).  |
| `checkwinsize`     | Update `$LINES`/`$COLUMNS` after each command (on by default on modern bash).|
| `huponexit`        | Send SIGHUP to all jobs when an interactive login shell exits.             |
| `histappend`       | Append to history file instead of overwriting on shell exit.               |
| `extdebug`         | Enables debugger/trace features; on under `--debugger`.                    |
| `compat31` … `compat50` | Restore behavior of older bash versions for compat.                   |

Usage:
- `shopt` — list everything with its state
- `shopt -s NAME` — set (enable)
- `shopt -u NAME` — unset (disable)
- `shopt -p` — reusable script output
- `shopt -o NAME` — restrict to options also settable via `set -o`

Invoke with `bash -O globstar -O extglob script.sh`.

---

## 6. Invocation matrix: startup files

The single most confusing part of bash is which files it sources when. The matrix:

|                                 | Login shell                                   | Non-login interactive   | Non-interactive         |
|---------------------------------|-----------------------------------------------|-------------------------|-------------------------|
| **Reads**                       | `/etc/profile`, then first of `~/.bash_profile`, `~/.bash_login`, `~/.profile` | `~/.bashrc` (and usually `/etc/bash.bashrc` via distro patch) | File pointed to by `$BASH_ENV` |
| **On exit, also reads**         | `~/.bash_logout`                              | —                       | —                       |
| **Suppress with**               | `--noprofile`                                 | `--norc`                | unset `BASH_ENV`        |
| **Override target file**        | —                                             | `--rcfile FILE`         | set `BASH_ENV` to FILE  |

"Login shell" = argv[0] starts with `-` **or** `--login`/`-l` was passed. `ssh host cmd` runs a non-login, non-interactive shell.

When invoked as `sh` (argv[0] == `sh`), bash enters a POSIX-leaning startup path: login shells read `/etc/profile` then `~/.profile`, interactive shells read `$ENV`, and nothing else is sourced.

`--posix` at runtime: similar POSIX-style startup file handling (only `$ENV`).

---

## 7. Restricted shell (`rbash`)

Start restricted by: invoking as `rbash`, using `-r`, or passing `--restricted`. In this mode bash disallows:

- `cd`
- Setting/unsetting `SHELL`, `PATH`, `ENV`, `BASH_ENV`
- Any command name containing `/`
- `.` or `source` with a path containing `/`
- `hash -p` with a path containing `/`
- Importing function definitions from the environment
- Honoring the `SHELLOPTS` environment variable
- Output redirection (`>`, `>>`, `<>`, `>&`, `&>`, `>|`)
- `exec` to replace the shell
- `enable -f` / `enable -d` / enabling disabled builtins
- `command -p`
- Turning off restricted mode (`set +r` is a no-op)

Restrictions apply **after** startup files — so a restrictive administrator can set `PATH` in `/etc/profile` before the lockdown activates. Important: `rbash` is **not** a security boundary. It's a speed bump. Real sandboxing uses namespaces, seccomp, chroot, containers, etc.

---

## 8. Environment variables bash reads at startup

The ones that affect *invocation* (not runtime — that list is huge). Selected:

| Variable          | Meaning at startup                                                   |
|-------------------|----------------------------------------------------------------------|
| `BASH_ENV`        | Filename sourced at the start of non-interactive shells.             |
| `ENV`             | POSIX equivalent; consulted when bash runs as `sh` or in posix mode. |
| `SHELLOPTS`       | Colon-list of `set -o` options to apply (read-only once bash starts).|
| `BASHOPTS`        | Colon-list of `shopt` options to apply.                              |
| `HISTFILE`        | History file path (default `~/.bash_history`).                       |
| `HISTSIZE`        | In-memory history entries.                                           |
| `HISTFILESIZE`    | On-disk history lines.                                               |
| `INPUTRC`         | readline init file (default `~/.inputrc`).                           |
| `HOME`            | Used for tilde expansion and as `cd`'s default target.               |
| `PATH`            | Command lookup path.                                                 |
| `POSIXLY_CORRECT` | If set at startup, equivalent to `--posix`.                          |
| `BASH_XTRACEFD`   | File descriptor to write `set -x` output to.                         |

---

## 9. Exit status

- `0` — success
- `1..125` — command-defined failure (`false` returns 1)
- `126` — command found but not executable
- `127` — command not found
- `128 + N` — killed by signal N (e.g. `130` for SIGINT, `137` for SIGKILL, `143` for SIGTERM)
- `255` — "exit status out of range" (bash itself treats `exit 256` as `exit 0` modulo 256, which is the source of several classic bugs)
- Bash itself on syntax error: non-zero (typically `2`)
- Builtins: `2` indicates incorrect usage (vs. runtime failure)

`$?` holds the exit status of the most recently completed foreground pipeline.

---

## 10. Built-in commands (bash 3.2+)

Every keyword below is a **builtin**, meaning it runs in-process. Aliasing any of these names won't substitute — use `command` or `builtin` to bypass.

```
:  .  [  alias  bg  bind  break  builtin  caller  cd  command  compgen
complete  continue  declare  dirs  disown  echo  enable  eval  exec  exit
export  false  fc  fg  getopts  hash  help  history  jobs  kill  let  local
logout  popd  printf  pushd  pwd  read  readonly  return  set  shift  shopt
source  suspend  test  times  trap  true  type  typeset  ulimit  umask
unalias  unset  wait
```

Reserved words (not builtins — parsed specially):

```
!  [[  ]]  {  }  case  do  done  elif  else  esac  fi  for  function  if
in  select  then  time  until  while  coproc  mapfile  readarray
```

Look up any of them with `help NAME`. `type NAME` disambiguates builtin vs. alias vs. function vs. file.

---

## 11. Common real-world invocation recipes

```bash
bash -c 'echo hi'                          # one-shot command
bash -c 'exit 42'; echo $?                 # control exit from outside
bash -c 'echo $0 $1' marker arg1           # custom $0 for error messages
bash -euo pipefail script.sh               # strict mode
bash -n script.sh                          # syntax-check only (no run)
bash -x script.sh                          # trace execution
BASH_XTRACEFD=9 bash -x script.sh 9>trace  # trace to a separate fd/file
bash -lc 'printenv'                        # login shell + single command (debug login env)
bash --rcfile /dev/null -i                 # interactive shell with no user rc
bash --noprofile --norc -i                 # clean interactive shell
bash -O globstar -O extglob script.sh      # opt into modern pattern features
env -i bash -l                             # login shell with empty environment
```

---

## 12. Authoritative resources

### Canonical documentation

- **Bash Reference Manual (GNU)** — the source of truth: <https://www.gnu.org/software/bash/manual/bash.html>
- **GNU Bash home** — release announcements, FAQ: <https://www.gnu.org/software/bash/>
- **`man bash`** locally — section structure used above; often more up-to-date than distro HTML mirrors.
- **Chet Ramey's bash page** (maintainer) — `NEWS`, `CHANGES`, articles: <https://tiswww.case.edu/php/chet/bash/bashtop.html>
- **bash source tarballs**: <https://ftp.gnu.org/gnu/bash/>
- **bash git (Savannah)**: <https://git.savannah.gnu.org/cgit/bash.git>
- **POSIX Shell & Utilities (IEEE Std 1003.1)**: <https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html>

### Practical guides

- **BashGuide (Greg Wooledge)** — best-practice intro: <https://mywiki.wooledge.org/BashGuide>
- **BashFAQ** — the canonical "why does X break" list: <https://mywiki.wooledge.org/BashFAQ>
- **BashPitfalls** — 50+ classic footguns with fixes: <https://mywiki.wooledge.org/BashPitfalls>
- **Advanced Bash-Scripting Guide (Mendel Cooper)** — broad, dated but useful: <https://tldp.org/LDP/abs/html/>
- **Google Shell Style Guide**: <https://google.github.io/styleguide/shellguide.html>

### Tooling

- **ShellCheck** — static analyzer; catches most of BashPitfalls automatically:
  - Site: <https://www.shellcheck.net/>
  - Source: <https://github.com/koalaman/shellcheck>
- **explainshell** — paste a command, get an annotated breakdown: <https://explainshell.com/>
- **bashdb** — source-level debugger: <https://bashdb.sourceforge.net/>
- **shfmt** — formatter: <https://github.com/mvdan/sh>

### Reading recommendations (by topic)

- **Quoting & word splitting**: BashGuide §"Arguments", BashFAQ/050
- **Arrays** and associative arrays (bash 4+): Reference Manual §"Arrays"
- **Process substitution, coprocesses, here-docs**: Reference Manual §"Redirections"
- **Traps, signals, job control**: `man bash` §`SIGNALS` and §`JOB CONTROL`
- **Readline / line editing**: `man 3 readline` and `~/.inputrc`; BashGuide §"Advanced"

---

## 13. Gotchas worth knowing

- **`sh` on macOS is bash 3.2** (from 2007), kept for GPLv2 licensing. Most `bash`-flavored scripts that rely on bash 4+ features (associative arrays, `${var,,}`, `readarray`, `coproc`, `**` globstar) will silently misbehave. Write either POSIX `sh` or require bash via `#!/usr/bin/env bash` and check `${BASH_VERSINFO[0]}`.
- **`set -e` is subtler than you think.** It doesn't trigger inside `if`/`while`/`||`/`&&` left operands, and command substitution doesn't inherit it without `shopt -s inherit_errexit` (bash 4.4+). See BashFAQ/105.
- **`read` on a piped loop runs in a subshell** — variable assignments don't escape. Fix with `shopt -s lastpipe` (only works with job control off) or a here-string.
- **`[` vs `[[`**: `[` is a POSIX utility with word-splitting on its arguments; `[[` is bash reserved-word syntax with no word splitting and richer operators (`=~`, `<`, `&&`). Prefer `[[` unless you're writing POSIX sh.
- **`echo` is not portable.** Flags, escape handling, and treatment of `-n` vary across shells and platforms. Use `printf '%s\n'` for anything non-trivial.
- **`-p` (privileged) is auto-enabled when real != effective uid.** `bash` will ignore your env and not source startup files under setuid. You almost certainly shouldn't be putting setuid on bash.

---

## 14. Version cheatsheet

| Bash | Year | Notable additions                                                                   |
|------|------|-------------------------------------------------------------------------------------|
| 2.05 | 2001 | `printf -v`, arithmetic `for`, redirection targets as variables                     |
| 3.0  | 2004 | `=~` regex match, process substitution, `{x..y}` range brace expansion              |
| 4.0  | 2009 | Associative arrays, `coproc`, `|&`, `**` globstar, `${var^^}`/`${var,,}`            |
| 4.2  | 2011 | `lastpipe`, `declare -g`, `$""` with quoted printf-style                            |
| 4.4  | 2016 | `${param@Q}` transforms, `mapfile` callback, `inherit_errexit`, `wait -n`           |
| 5.0  | 2019 | `BASH_ARGV0`, `history -d` negative, nameref improvements, `EPOCHSECONDS`           |
| 5.1  | 2020 | `SRANDOM` (true random), `wait -p VAR`, `ulimit -R`                                 |
| 5.2  | 2022 | `varredir_close` shopt, more `compat*` levels, POSIX-mode fixes                     |

Runtime check:

```bash
(( BASH_VERSINFO[0] >= 4 )) || { echo "bash 4+ required"; exit 2; }
```

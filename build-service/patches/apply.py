#!/usr/bin/env python3
"""Apply MeshCore source patches needed by build options.

Each patch is a targeted, idempotent edit rather than a diff, so it keeps
working when upstream shifts surrounding lines. A patch whose anchor is missing
is a hard error: silently skipping it would produce firmware that ignores the
option the user asked for, which is worse than failing the build.
"""
import sys
import pathlib

src = pathlib.Path(sys.argv[1])


def sub_once(rel, old, new, why):
    p = src / rel
    text = p.read_text()
    if new in text:
        print(f"[patch] {rel}: already applied")
        return
    n = text.count(old)
    if n != 1:
        sys.exit(f"[patch] {rel}: expected 1 occurrence of anchor, found {n} ({why})")
    p.write_text(text.replace(old, new, 1))
    print(f"[patch] {rel}: {why}")


# No patches are currently needed: every option is expressible as a compiler
# flag against unmodified upstream source. That is the preferred state — a patch
# is a maintenance liability that has to survive every upstream release — so
# reach for one only when a flag genuinely cannot do the job.
#
# The buzzer option used to patch NodePrefs.h to make the saved preference's
# factory default a build flag. It was dropped once -UPIN_BUZZER proved both
# stronger (nothing can beep, rather than asking it not to) and patch-free.

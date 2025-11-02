# RB-OKVS Patches

This directory contains patches for the `rb-okvs` submodule that need to be applied during setup.

## Patches

- **0001-feature-gate-fix-and-tests.patch**: Fix feature gate for test feature and add tests directory
  - Changes `#![feature(test)]` to `#![cfg_attr(test, feature(test))]` for better compatibility
  - Adds `tests/float64_use_case.rs` for silhouette-db specific tests

## Applying Patches

Patches are automatically applied when running:

```bash
./scripts/apply-patches.sh
```

Or manually:

```bash
cd third_party/rb-okvs
git apply ../../patches/rb-okvs/0001-feature-gate-fix-and-tests.patch
```

## Base Commit

Patches are based on commit `1fcf747` from the upstream `felicityin/rb-okvs` repository.

## Notes

- Patches should be applied in order (numbered patches)
- If a patch fails to apply, it may already be applied or the submodule may be on a different commit
- Check patch status with: `git apply --check patches/rb-okvs/*.patch`

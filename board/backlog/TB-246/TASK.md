# TB-246: Regenerate darwin/Assets.car on working Xcode env

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** branding,build,macos
**Branch:** —

## Goal

TB-202 dropped gui/build/darwin/Assets.car because actool's plugin loading was broken in the dev environment that completed the rebrand. macOS currently falls back to icons.icns which is acceptable, but Assets.car is the modern asset-catalog output and should be regenerated on a working Xcode install via `wails3 task common:generate:icons` (or by running actool against gui/build/appicon.icon directly) and committed so packaged macOS builds get the full asset catalog.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-19: Created

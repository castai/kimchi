# Status line

Use `/customize-status-line` to pin fields and choose up to three native status rows. Select **Status rows** and press Space or Enter to cycle 1 → 2 → 3 → 1. One row remains the default; extra rows are used only when needed.

The layout packs fields before shortening or hiding lower-priority fields. Permissions and model stay first, and the existing compaction priorities still apply at narrow widths.

Ferment V2 has a separate field from Ferment. A set objective appears automatically with its state; pinning the field keeps an idle placeholder visible when no objective exists. Checking appears while completion is evaluated; paused and blocked runs include a resume hint.

For custom `statusLine.command` scripts, the row setting applies to the native controls below the script. The script's own lines are preserved, and changing pins or rows keeps the command and other script settings intact.

Settings are saved under `statusLine` in `~/.config/kimchi/harness/settings.json`:

```json
{
  "statusLine": {
    "pinned": ["thinking", "agents", "context", "usage", "ferment-v2"],
    "lines": 2
  }
}
```

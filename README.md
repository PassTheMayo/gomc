# Minecraft Server in Go

A Minecraft: Java Edition server for **Minecraft 26.3**, written from scratch in Go. Connect with the normal, unmodified Minecraft launcher. No mods or special client needed.

> [!NOTE]
> This project was written entirely by AI as a proof of concept. It isn't a complete replacement for the official server, but everything it does is built to match vanilla Minecraft and to work with the vanilla client.

## What You Can Do

- **Join with your Minecraft account.** Accounts are verified with Mojang like on any official server, and everyone sees each other's skins.
- **Explore a generated world** with oceans, beaches, hills, mountains, forests, deserts and snowy biomes across 20 biome types. A classic superflat world is also available.
- **Play with friends.** You can see other players walk, turn, sneak, sprint and swing their arms, and they show up in the tab list.
- **Build.** Place and break blocks. Stairs, logs, doors, beds, torches, signs and other blocks face the way you'd expect. Everyone nearby sees your changes.
- **Use the creative inventory.** Your inventory, including enchanted and renamed items, is saved between sessions, along with your selected hotbar slot.
- **Middle-click to pick blocks**, just like vanilla.
- **Drop items** with the drop key, and swap items to your off hand with the swap key.
- **Come back later.** The world, your position and your inventory are saved automatically every five minutes and when the server stops.

## What Doesn't Work Yet

This is an early proof of concept, so much of Minecraft is still missing:

- No mobs, animals or villagers.
- No health, hunger or damage, and no way to die.
- No chat. Commands still work.
- Blocks don't react to anything. Water and lava don't flow, sand doesn't fall, doors don't open, and torches float when the block under them is broken.
- Broken blocks don't drop items, so survival mode is mostly for walking around.
- No crafting, furnaces or chest storage.
- Only the overworld exists. There's no Nether or End.
- No caves, ores or villages in the generated world.

The full checklist of what is and isn't done is in [TODO.md](TODO.md).

## Hosting a Server

You'll need [Go 1.27](https://go.dev/dl/) and players on Minecraft Java Edition **26.3**.

```sh
cp config.example.yml config.yml
make run
```

Players can then join at `localhost:25565`, or at your computer's address from other machines. Stop the server with `Ctrl+C` so it saves the world before closing.

### Settings

Edit `config.yml` to change how the server works:

| Setting | Default | What it does |
| --- | --- | --- |
| `host` | `0.0.0.0` | Network address to listen on |
| `port` | `25565` | Port players connect to |
| `motd` | | Message shown in the server list |
| `max_players` | `20` | How many players can be online at once |
| `online_mode` | `true` | Check accounts with Mojang. Turn it off only for offline LAN play |
| `compression_threshold` | `256` | Compress network traffic above this size; `-1` turns compression off |
| `view_distance` | `10` | How many chunks players can see (2–32) |
| `simulation_distance` | `10` | Simulation distance shown to players (2–32) |
| `game_mode` | `creative` | `survival`, `creative`, `adventure` or `spectator` |
| `operators` | none | Players who can use operator commands |
| `server_links` | none | Links in the pause menu, each with a `label` and a `url` (see below) |
| `world.directory` | `world` | Folder the world is saved in |
| `world.seed` | `0` | World seed; `0` picks a random one |
| `world.generator` | `terrain` | `terrain` for normal worlds or `flat` for superflat |

The seed and generator only apply when a world is first created. To change them, use a new `world.directory` or delete the old world folder.

Any setting can also be set with an environment variable of the same name in capitals, such as `PORT=25566`. For nested settings, join the parts with `_`, as in `WORLD_SEED`. List operators separated by commas: `OPERATORS=Alice,Bob`. Server links are `label=url` pairs separated by commas: `SERVER_LINKS=website=https://example.com,report_bug=https://example.com/issues`.

### Server Links

Server links appear under **Server Links** in the pause menu. A link's label can be one of Minecraft's built-in names, which players see translated into their own language: `report_bug`, `community_guidelines`, `support`, `status`, `feedback`, `community`, `website`, `forums`, `news` or `announcements`. Any other label is shown as written. Links must start with `http://` or `https://`.

```yaml
server_links:
  - label: website
    url: https://example.com
  - label: Live map
    url: https://example.com/map
```

### Server Icon

To show an icon in the server list, put a 64×64 PNG named `server-icon.png` next to `config.yml`. Without one, the server uses the world's `icon.png` if it has one.

> [!WARNING]
> With `online_mode` off, anyone can join under any name, including an operator's.

### Commands

Commands tab-complete in the chat box like vanilla commands.

| Command | Who can use it | What it does |
| --- | --- | --- |
| `/list` | Everyone | Shows who is online |
| `/list uuids` | Everyone | Shows who is online, with each player's UUID |
| `/pregen <radius>` | Operators | Generates the world ahead of time for `radius` chunks around you, so exploring it later is faster |
| `/pregen cancel` | Operators | Stops a running pre-generation |

Pre-generation runs in the background and shows its progress above your hotbar.

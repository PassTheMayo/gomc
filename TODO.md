# TODO

Everything a server needs to match vanilla Minecraft: Java Edition, grouped by area and sorted alphabetically within each group. `[x]` marks implemented features. An item with sub-items is only checked once all of them are done.

## Networking

- [ ] **Bundle packets**: groups packets so the client applies them together
- [x] **Compression**: zlib for packets above a configurable threshold
- [ ] **Cookies**: stores and requests cookies on the client
- [ ] **Custom report details**: extra details added to client crash reports
- [x] **Encryption**: AES/CFB8 after the online-mode key exchange
- [x] **Handshake**: reads the intent and rejects clients on a different protocol version
- [x] **Keep-alive**: pings every 15 s, kicks after 30 s without a reply and smooths latency
- [ ] **Legacy server list ping**: replies to pre-1.7 clients
- [x] **Packet framing**: VarInt-length frames with a 2 MiB size limit
- [ ] **Packet rate limiting**: kicks clients that flood packets
- [ ] **Play-phase reconfiguration**: sends players back to the configuration phase
- [x] **Read timeouts**: drops connections that stop sending
- [x] **Server links**: configurable links in the pause menu
- [x] **Server list favicon**: `server-icon.png`, or the world's icon, in the server list and refreshed on join
- [x] **Server list ping**: answers ping requests with a pong
- [x] **Server list player sample**: names of up to 12 online players on hover
- [x] **Server list status**: MOTD, version, online and maximum player counts
- [x] **Transfer intent**: accepts connections that were transferred from another server
- [ ] **Transfer packet**: sends players to another server

## Login and Authentication

- [x] **Duplicate logins**: kicks the older session of the same account
- [ ] **IP bans**: banned addresses can't join
- [ ] **Login plugin messages**: custom query and answer during login
- [ ] **Name bans**: banned players can't join and see the reason
- [x] **Offline mode**: offline UUIDs derived from the player name
- [x] **Online mode**: RSA key exchange and Mojang session server verification
- [x] **Player limit**: refuses logins when the server is full
- [ ] **Prevent proxy connections**: rejects players whose IP differs from the session server's
- [ ] **Secure chat profile keys**: verifies the chat signing keys players send
- [x] **Skins**: profile properties passed on to all players
- [ ] **Whitelist**: only listed players may join

## Configuration Phase

- [x] **Client information**: locale, view distance and skin layers are stored
- [ ] **Custom registry contents**: full entries for data-pack registries the client doesn't know
- [ ] **Experimental feature flags**: optional experimental features
- [x] **Feature flags**: vanilla feature set enabled
- [x] **Known packs**: requires the vanilla core pack and sends registries without their contents
- [ ] **Resource packs**: server resource pack prompt, required packs and status handling
- [x] **Server brand**: sent on the `minecraft:brand` channel
- [x] **Synchronized registries**: all 32 registries the client expects
- [x] **Tags**: every vanilla tag, including the item tags referenced by vanilla data

## Server Administration

- [x] **Configuration file**: `config.yml` with environment variable overrides
- [ ] **Console input**: commands typed into the server console
- [ ] **Difficulty**: peaceful, easy, normal and hard, with the difficulty lock
- [ ] **Game rules**: all vanilla game rules and how they change behaviour
- [x] **Graceful shutdown**: Ctrl+C saves the world before exiting
- [ ] **Hardcore mode**: a single life and hardcore hearts
- [ ] **Idle timeout**: kicks players who stay idle too long
- [ ] **Operator permission levels**: levels 1–4 that limit commands
- [x] **Operators**: names in the configuration may run operator commands
- [ ] **PvP toggle**: turns player damage from other players on or off
- [ ] **Query protocol**: GameSpy4 server status queries
- [ ] **RCON**: remote console protocol
- [ ] **Server performance metrics**: TPS and tick time reporting
- [ ] **Spawn protection**: non-operators can't edit blocks near spawn
- [x] **Structured logging**: `slog` output for connections, joins and errors
- [ ] **Watchdog**: stops the server when a tick hangs past the maximum tick time

## Ticking and Performance

- [x] **Parallel chunk generation**: chunks generated on every CPU core
- [x] **Per-player chunk streaming**: each player streams chunks on its own goroutine
- [ ] **Random ticks**: random block ticks per section, scaled by the `randomTickSpeed` rule
- [ ] **Scheduled ticks**: block and fluid ticks scheduled for later
- [ ] **Simulation distance**: only blocks and entities near players are ticked (currently only reported to clients)
- [ ] **Tick control**: freezing, stepping and sprinting the game with `/tick`
- [x] **Tick loop**: 20 TPS for entity tracking, keep-alives and time

## Persistence

- [ ] **Anvil import and export**: reading and writing vanilla worlds
- [x] **Autosave**: changed chunks, players and level data every five minutes
- [ ] **Block entity data**: container contents, sign text, banner patterns and other block entity NBT
- [ ] **Entities**: mobs, items, vehicles and other entities in chunks
- [ ] **Game rules and world settings**: saved with the level
- [x] **Level data**: seed, game time and generator
- [x] **Player data**: position, rotation, selected hotbar slot and inventory
  - [x] **Version-proof items**: items saved by name, with components dropped after a protocol change
- [ ] **Player state**: health, hunger, experience, effects, ender chest, spawn point and game mode
- [x] **Region files**: the server's own format of 32×32 chunk regions
  - [x] **Append-only records**: a crash never corrupts a region
  - [x] **Compaction**: rewrites regions with too many superseded records
  - [x] **CRC32 checksums**: corrupted chunks are detected and regenerated from the seed
  - [x] **Names instead of IDs**: block states and biomes saved by name to survive version changes
- [x] **Save on shutdown**: everything is written on a graceful stop
- [ ] **Statistics and advancements**: per-player progress

## Chunks

- [x] **Biomes**: biome data sent with chunks
- [x] **Block entities in chunks**: types sent with chunk data so blocks like chests render
- [x] **Chunk recentering**: the client's chunk cache center follows the player
- [x] **Chunk streaming**: nearest chunks first in batches paced by client acknowledgements
- [ ] **Chunk tickets**: loading levels from players, portals and entities
- [x] **Chunk unloading**: chunks are forgotten as players move away
- [ ] **Forced chunks**: chunks kept loaded with `/forceload`
- [x] **Heightmaps**: motion blocking and world surface maps sent with chunks
- [ ] **Multi-block updates**: section-wide changes in one packet
- [x] **Paletted containers**: block states and biomes packed per section
- [x] **Pre-generation**: `/pregen` writes chunks to disk in the background
- [ ] **Spawn chunks**: chunks around spawn stay loaded and ticked

## World Generation

- [ ] **3D biomes**: cave biomes like lush caves, dripstone caves and the deep dark
- [ ] **All overworld biomes**: the roughly 65 vanilla overworld biomes
- [ ] **All tree types**: jungle, acacia, dark oak, cherry, mangrove, pale oak, azalea, huge mushrooms and fungi
- [ ] **Aquifers**: underground water and lava levels
- [ ] **Caves**: cheese, spaghetti and noodle caves, plus carvers
- [ ] **Data-pack world generation**: custom noise settings, biomes and structures
- [ ] **End generation**: the main island, outer islands, chorus plants and gateways
- [ ] **Features**: lakes, springs, geodes, fossils, icebergs, dripstone, boulders and other placed features
- [x] **Flat generator**: classic superflat
- [ ] **Nether generation**: terrain, biomes and features
- [ ] **Ores**: ore veins and blobs by height and biome
- [x] **Safe spawn**: finds a dry spawn point when the world is created
- [ ] **Structures**
  - [ ] **Ancient cities**
  - [ ] **Bastion remnants**
  - [ ] **Buried treasure**
  - [ ] **Desert and jungle temples**
  - [ ] **End cities**
  - [ ] **Igloos**
  - [ ] **Mineshafts**
  - [ ] **Nether fortresses**
  - [ ] **Ocean monuments**
  - [ ] **Ocean ruins**
  - [ ] **Pillager outposts**
  - [ ] **Ruined portals**
  - [ ] **Shipwrecks**
  - [ ] **Strongholds**
  - [ ] **Swamp huts**
  - [ ] **Trail ruins**
  - [ ] **Trial chambers**
  - [ ] **Villages**
  - [ ] **Woodland mansions**
- [x] **Terrain generator**: the server's own overworld terrain
  - [x] **Beaches**: sand along coastlines
  - [x] **Biomes**: 20 biomes chosen by height, temperature and humidity
  - [x] **Continents and oceans**: land masses and seas from noise
  - [x] **Hills and mountains**: rolling hills and ridged mountains
  - [x] **Snow and ice**: snow cover and frozen water in cold biomes
  - [x] **Trees**: oak, birch and spruce
  - [x] **Vegetation**: cacti, grass and flowers
- [ ] **Vanilla-accurate terrain**: the density-function noise router, so seeds match vanilla
- [ ] **World presets**: amplified, large biomes, single biome and debug worlds

## Lighting

- [ ] **Block light**: light from torches, lava, glowstone and other emitting blocks
- [ ] **Light updates**: light recalculated and sent when blocks change
- [ ] **Light-dependent mechanics**: mob spawning, crop growth and ice melting by light level
- [ ] **Sideways sky light**: sky light spreading under overhangs and into caves
- [x] **Sky light**: falls straight down from the sky in every chunk sent

## Time, Weather and Environment

- [x] **Day cycle**: time and the overworld clock sent to clients
- [x] **Game time**: advances every tick and is saved
- [ ] **Lightning**: strikes, fires, charged creepers and skeleton horse traps
- [ ] **Sleeping**: skipping the night when enough players sleep
- [ ] **Snow and ice from weather**: snow layers and ice forming during precipitation
- [ ] **Time rules**: stopping the day cycle with game rules
- [ ] **Weather**: rain, snow and thunderstorms with their timers
- [ ] **World border**: size, center, shrinking, warnings and damage

## Dimensions

- [ ] **Changing dimensions**: respawn packets and moving players between levels
- [ ] **Custom dimensions**: dimensions from data packs
- [ ] **End gateways**: gateways spawned after dragon fights and their teleports
- [ ] **End portals**: filling frames with eyes of ender and teleporting to the End
- [ ] **End**: dimension with the end spawn platform
- [ ] **Ender dragon fight**: dragon, crystals, exit portal and dragon egg
- [ ] **Nether portals**: lighting frames, linking and creating exit portals
- [ ] **Nether**: dimension with 8:1 coordinate scaling
- [x] **Overworld**: the only dimension that exists

## Players

- [ ] **Abilities**: flight, invulnerability, instant build and fly or walk speeds
- [ ] **Adventure mode rules**: blocks may only be broken or placed as items allow
- [ ] **Beds**: sleeping, setting spawn and exploding outside the overworld
- [ ] **Breathing**: air supply underwater, drowning and water breathing
- [ ] **Changing game modes**: switching game modes in play and updating abilities
- [ ] **Death**: death messages, dropped inventory and the death screen
- [ ] **Experience**: levels, orbs and experience from blocks, mobs and trading
- [x] **Game modes**: survival, creative, adventure or spectator from the configuration
- [ ] **Health**: damage, natural regeneration and the health bar
- [ ] **Hunger**: food level, saturation, exhaustion and starvation
- [x] **Join and leave messages**: yellow chat messages for everyone
- [x] **Join sequence**: login, spawn position, teleport, time and chunk loading
- [x] **Latency updates**: tab list latency refreshed periodically
- [ ] **Main hand**: left-handed players shown to others with their main hand
- [ ] **Player list customization**: header, footer, display names and order
- [ ] **Respawning**: respawn at spawn, beds or respawn anchors
- [x] **Saved position**: returning players appear where they left
- [x] **Spawn point**: new players appear at the world spawn
- [ ] **Spectator mode**: flying through blocks and spectating entities
- [ ] **Status effects**: potions and beacons, with effect timers and their behaviour
- [x] **Tab list**: entries with skins, game mode and latency, updated as players join and leave

## Movement

- [ ] **Block effects on movement**: soul sand, honey, cobwebs, powder snow and bubble columns
- [ ] **Climbable blocks**: ladders, vines and scaffolding rules
- [ ] **Elytra flight**: gliding and firework boosts
- [ ] **Fall damage**: damage from falling, softened by water, hay, slime and beds
- [x] **Movement state**: sneaking, sprinting and flying tracked from client input
- [ ] **Movement validation**: rejects moving too fast, through blocks or into the air
- [ ] **Poses**: swimming, crawling, gliding, sleeping and spin attacks
- [x] **Position and rotation**: movement packets applied with NaN, infinity and world-bound checks
- [ ] **Riding**: mounting, steering and dismounting entities
- [x] **Teleport confirmation**: movement is ignored until the client confirms a teleport
- [ ] **Void**: damage below the world

## Entities

- [ ] **Animations**: damage tilt, critical hits and other entity events
- [x] **Arm swings**: other players see swings when a player punches, mines or places blocks, with spears stabbing
- [ ] **Attributes**: health, speed, attack damage, scale and other attributes with modifiers
- [ ] **Decorations**: armor stands, item frames, paintings, leash knots and end crystals
- [ ] **Display entities**: block, item and text displays, and interaction entities
- [x] **Entity IDs**: unique IDs for every entity
- [x] **Entity metadata**: shared flags and poses for sneaking and sprinting, and skin layers
- [x] **Entity movement**: relative moves, rotations, head rotations and periodic position syncs
- [ ] **Entity physics**: gravity, drag, collisions and fluids
- [ ] **Equipment**: held items and armor visible to other players
- [ ] **Experience orbs**: spawning, merging and attraction to players
- [ ] **Falling blocks**: sand, gravel, anvils and pointed dripstone
- [ ] **Item entities**: dropped items, merging, despawning and pickup
- [ ] **Other entities**: area effect clouds, evoker fangs, fireworks, fishing bobbers, eyes of ender, lightning and markers
- [x] **Player entities**: spawned for players within view distance and removed outside it
- [ ] **Primed TNT**: fuse and explosion
- [ ] **Projectiles**: arrows, tridents, snowballs, eggs, ender pearls, potions, fireballs and wind charges
- [ ] **Vehicles**: boats, rafts and every minecart type
- [ ] **Velocity**: motion packets for knockback, explosions and launches

## Mobs

- [ ] **AI**: goals, targeting, pathfinding and navigation for land, water and air
- [ ] **Bosses**: ender dragon, wither, elder guardian and warden
- [ ] **Breeding**: love mode, baby mobs and growing up
- [ ] **Despawning**: distance-based despawning and persistence rules
- [ ] **Hostile mobs**: blazes, bogged, breezes, creakings, creepers, drowned, guardians, hoglins, husks, illagers, magma cubes, phantoms, piglin brutes, ravagers, shulkers, silverfish, skeletons, slimes, spiders, strays, vexes, witches, zoglins, zombies and others
- [ ] **Iron golems**: village golem spawning and player-built golems
- [ ] **Mob griefing**: endermen moving blocks, creepers and ghasts destroying terrain
- [ ] **Mob interactions**: shearing, milking, leashing, saddling, name tags and buckets
- [ ] **Mob loot**: drops and experience from loot tables
- [ ] **Mob variants**: colors and variants for sheep, cats, wolves, frogs, cows, pigs, chickens and others
- [ ] **Natural spawning**: by biome, light level, mob caps and spawn rules
- [ ] **Neutral mobs**: bees, dolphins, endermen, foxes, goats, iron golems, llamas, pandas, piglins, polar bears, wolves and others
- [ ] **Passive mobs**: allays, armadillos, axolotls, bats, camels, cats, chickens, cows, fish, frogs, horses, mooshrooms, ocelots, parrots, pigs, rabbits, sheep, sniffers, squids, striders, turtles, villagers and others
- [ ] **Raids**: bad omen, raid waves and the hero of the village effect
- [ ] **Spawners**: monster spawners and trial spawners
- [ ] **Taming**: wolves, cats, parrots, horses and other tameable mobs
- [ ] **Trading**: offers, restocking, price changes and wandering traders
- [ ] **Villagers**: professions, workstations, schedules, gossip and breeding
- [ ] **Zombie sieges and conversions**: zombie villagers, drowned and other conversions

## Combat and Damage

- [ ] **Armor**: armor points, toughness and damage reduction
- [ ] **Attack cooldown**: damage scaled by the attack strength meter
- [ ] **Critical hits and sweeping**: falling hits and sword sweeps
- [ ] **Damage types**: all vanilla damage types and their sources
- [ ] **Death messages**: messages for every damage source
- [ ] **Environmental damage**: fire, lava, cactus, berry bushes, suffocation, freezing and magma
- [ ] **Explosions**: block destruction, damage, knockback and drops
- [ ] **Invulnerability frames**: brief immunity after taking damage
- [ ] **Knockback**: from attacks, explosions and projectiles
- [ ] **Mace and spear attacks**: smash attacks and stabbing
- [ ] **Melee attacks**: attacking entities, with reach checks
- [ ] **Shields**: blocking, disabling with axes and durability
- [ ] **Totems of undying**: preventing death

## Inventory and Containers

- [ ] **Armor slots**: equipping armor and elytra, and the curse of binding
- [ ] **Bundles**: storing items inside bundles
- [ ] **Container menus**: chests, barrels, shulker boxes, hoppers, dispensers and other containers
- [x] **Creative inventory**: slots set from the creative inventory are tracked, with item components
- [ ] **Cursor item**: the item held on the cursor
- [ ] **Double chests**: two chests combining into one inventory
- [x] **Dropping with the drop key**: removes one item or the whole stack from the server's copy
- [ ] **Ender chests**: per-player storage
- [x] **Hotbar selection**: the selected slot is tracked and restored on join
- [x] **Inventory sync on join**: the saved inventory is sent to the client
- [ ] **Item drops**: dropped items appear in the world
- [ ] **Item pickup**: collecting nearby items
- [x] **Off-hand swapping**: the swap key moves items between hands
- [x] **Picking blocks**: middle-click follows vanilla
  - [x] **Creative picking**: creative players get the item when they don't have it
  - [x] **Hotbar slot choice**: empty slots first, then slots without enchanted items
  - [x] **Item mapping**: crops as seeds, wall blocks as their item, potted plants as the plant and so on
  - [x] **Selecting**: a matching stack in the hotbar is selected
  - [x] **Swapping**: a matching stack in the main inventory is swapped into the hotbar
- [ ] **Picking entities**: middle-clicking entities gives their spawn egg or item
- [ ] **Picking with data**: Ctrl+middle-click copies block entity data
- [ ] **Survival inventory clicks**: picking up, placing, quick moving, swapping, cloning, throwing and dragging

## Items

- [ ] **Archaeology**: brushing suspicious sand and gravel
- [x] **Block items**: place their block in creative and survival
- [ ] **Bone meal**: growing plants and spreading grass
- [ ] **Books**: writing and signing books, and reading them on lecterns
- [ ] **Bows, crossbows and tridents**: charging, firing and loyalty, riptide and multishot behaviour
- [ ] **Buckets**: picking up and placing fluids, mobs and powder snow
- [ ] **Compasses**: spawn, lodestone and recovery compasses
- [ ] **Durability**: tools and armor wearing out and breaking
- [ ] **Ender pearls and eyes of ender**: teleporting and locating strongholds
- [ ] **Fire starters**: flint and steel and fire charges
- [ ] **Fishing**: rods, bobbers, loot and lures
- [ ] **Food**: eating, nutrition and food effects
- [ ] **Goat horns**: playing horn sounds
- [x] **Item components**: kept as sent by clients and sent back unchanged
- [ ] **Item consumption**: using up placed blocks and items in survival
- [ ] **Item cooldowns**: ender pearls, chorus fruit, shields and wind charges
- [ ] **Item validation**: rejecting invalid stacks and components from clients
- [ ] **Leads, saddles and name tags**: using them on mobs
- [ ] **Maps**: exploring, scaling, locking, banner markers and explorer maps
- [ ] **Music discs**: playing records in jukeboxes
- [ ] **Potions**: drinking, splash and lingering potions, and tipped arrows
- [ ] **Spawn eggs**: spawning mobs, including from spawners
- [ ] **Tools**: mining speed, correct-tool drops and tool-specific actions
- [ ] **Using items**: right clicks in the air and item use timers

## Block Placement

- [x] **Acknowledgements**: block change sequences acknowledged to clients
- [x] **Broadcasting**: changes sent to everyone who can see the chunk
- [ ] **Connecting blocks**: fences, walls, panes, iron bars, redstone wire and chests
- [ ] **Interactable blocks**: right-clicking doors, levers, buttons, trapdoors, gates and other usable blocks
- [x] **Orientation**: vanilla rules for logs, stairs, slabs, doors, beds, buttons and other shapes
- [ ] **Placement support**: blocks that need support, like torches and flowers, can only go where they survive
- [x] **Placing blocks**: right-clicking a face with a block item
- [x] **Player collision**: solid blocks can't be placed inside players
- [x] **Reach check**: blocks out of reach are refused
- [x] **Replaceable blocks**: grass, fluids and similar blocks are replaced
- [ ] **Sign editing**: editing text, dyes, glow ink and waxing
- [ ] **Slab merging**: two slabs combining into a double slab
- [ ] **Stacking blocks**: snow layers, candles, sea pickles, turtle eggs and similar blocks
- [x] **Two-block objects**: both halves of doors, beds and tall plants are placed
- [x] **Wall variants**: torches, signs, banners, heads and coral fans attach to walls
- [x] **Waterlogging**: blocks placed in water become waterlogged
- [x] **World height**: placements outside the build limits are refused

## Block Breaking

- [ ] **Break effects for others**: crack animations, particles and sounds
- [x] **Creative breaking**: instant on the first hit
- [ ] **Creative weapon rule**: swords, tridents and maces can't break blocks in creative
- [ ] **Dig speed validation**: digging times checked against tools and effects
- [ ] **Drops**: loot tables, silk touch, fortune and experience
- [x] **Instant-break blocks**: plants and similar blocks break on the first hit in survival
- [x] **Survival breaking**: when the client finishes digging
- [x] **Two-block objects**: breaking either half removes both
- [ ] **Unsupported blocks**: torches, plants and other attached blocks breaking with their support
- [x] **Waterlogged blocks**: leave water behind

## Block Behaviour

- [ ] **Beehives**: bees, honey levels and harvesting
- [ ] **Bells**: ringing and raid highlighting
- [x] **Block properties**: light opacity, solidity, collision, fluids and replaceability
- [x] **Block states**: all 35,723 states of 26.3, with properties and defaults
- [ ] **Cauldrons**: filling with water, lava and powder snow, and washing items
- [ ] **Composters**: composting items into bone meal
- [ ] **Concrete powder**: hardening next to water
- [ ] **Copper aging**: oxidation, waxing and scraping
- [ ] **Coral**: dying out of water
- [ ] **Doors, trapdoors and gates**: opening and closing
- [ ] **Farmland**: hydration, trampling and turning back to dirt
- [ ] **Fire**: spreading, burning blocks, burning out and soul fire
- [ ] **Fluid interactions**: cobblestone, stone, obsidian and basalt forming
- [ ] **Fluids**: water and lava flow, sources, levels and waterlogged blocks
- [ ] **Gravity blocks**: sand, gravel, concrete powder, anvils and scaffolding
- [ ] **Leaf decay**: leaves far from logs disappearing
- [ ] **Melting and freezing**: ice, snow and frosted ice
- [ ] **Neighbour updates**: shape updates and neighbour changes propagating between blocks
- [ ] **Note blocks**: instruments and pitches
- [ ] **Plant growth**: crops, saplings, sugar cane, cactus, bamboo, kelp, vines and stems
- [ ] **Pointed dripstone**: growth, dripping and falling
- [ ] **Sculk**: sensors, shriekers and catalysts
- [ ] **Spreading blocks**: grass, mycelium, nylium, moss and sculk
- [ ] **TNT**: ignition and explosions

## Block Entities

- [ ] **Banners**: patterns
- [ ] **Beacons**: pyramids, beams and effects
- [ ] **Block entity data**: storing and updating block entity NBT
- [x] **Block entity types**: sent with chunks so blocks with block entities render
- [ ] **Brewing stands**: brewing potions with blaze powder
- [ ] **Campfires**: cooking food and signal smoke
- [ ] **Chests, barrels and shulker boxes**: inventories, opening animations and loot tables
- [ ] **Chiseled bookshelves**: storing books
- [ ] **Command blocks**: impulse, chain and repeating modes
- [ ] **Conduits**: activation and effects
- [ ] **Crafters**: automated crafting
- [ ] **Creaking hearts**: spawning creakings
- [ ] **Decorated pots**: sherds and storage
- [ ] **Furnaces, blast furnaces and smokers**: smelting, fuel and experience
- [ ] **Heads**: player profiles and note block sounds
- [ ] **Hoppers, droppers and dispensers**: moving items and dispensing behaviours
- [ ] **Jukeboxes**: playing discs and comparator output
- [ ] **Lecterns**: books and page turning
- [ ] **Signs**: text on both sides
- [ ] **Spawners**: mob spawning settings
- [ ] **Structure and jigsaw blocks**: saving, loading and generating structures
- [ ] **Test blocks**: game test blocks
- [ ] **Trial spawners and vaults**: trial chamber rewards

## Redstone

- [ ] **Observers**: detecting block updates
- [ ] **Pistons**: pushing and pulling blocks, slime and honey, and quasi-connectivity
- [ ] **Power sources**: levers, buttons, pressure plates, tripwire, daylight detectors, target blocks and redstone blocks
- [ ] **Power**: signal strength, strong and weak power, and update order
- [ ] **Rails**: powered, detector and activator rails
- [ ] **Redstone torches**: inversion and burnout
- [ ] **Redstone wire**: connections and signal decay
- [ ] **Redstone-powered blocks**: lamps, doors, note blocks, copper bulbs, TNT and others
- [ ] **Repeaters and comparators**: delays, locking, comparison and subtraction

## Crafting and Processing

- [ ] **Anvils**: repairing, renaming, combining enchantments and anvil damage
- [ ] **Brewing**: potion recipes
- [ ] **Cartography tables**: copying, scaling and locking maps
- [ ] **Crafting**: the 2×2 player grid and 3×3 crafting tables
- [ ] **Enchanting tables**: offers, bookshelves and lapis cost
- [ ] **Enchantment effects**: every vanilla enchantment from its data-driven definition
- [ ] **Grindstones**: removing enchantments and repairing
- [ ] **Looms**: banner patterns
- [ ] **Recipe book**: unlocking recipes, filling the grid and book settings
- [ ] **Recipes**: the recipe registry and recipe types from data packs
- [ ] **Smelting**: furnaces, fuel and experience
- [ ] **Smithing**: netherite upgrades and armor trims
- [ ] **Stonecutters**: stonecutting recipes

## Chat and Commands

- [x] **Action bar messages**: overlay text above the hotbar
- [ ] **Chat limits**: spam throttling and message length checks
- [ ] **Chat types**: decorations for chat, emotes, whispers and team messages
- [x] **Command permissions**: operator-only commands hidden from other players
- [ ] **Command suggestions**: server-side suggestions for arguments like player names
- [x] **Command tree**: sent to clients for tab completion and argument checking
- [ ] **Functions**: data-pack functions, tags, macros and `/function`
- [ ] **Player chat**: chat messages broadcast to players
- [x] **`/pregen`**: server-specific chunk pre-generation with progress and cancelling
- [ ] **Signed chat**: chat sessions, signatures, last-seen acknowledgements and reporting
- [x] **System messages**: chat messages from the server
- [ ] **Target selectors**: `@a`, `@e`, `@n`, `@p`, `@r` and `@s` with filters
- [ ] **Vanilla commands**
  - [ ] **`/advancement`**
  - [ ] **`/attribute`**
  - [ ] **`/ban`, `/ban-ip`, `/banlist`, `/pardon` and `/pardon-ip`**
  - [ ] **`/bossbar`**
  - [ ] **`/clear`**
  - [ ] **`/clone`**
  - [ ] **`/damage`**
  - [ ] **`/data`**
  - [ ] **`/datapack`**
  - [ ] **`/debug`**
  - [ ] **`/defaultgamemode`**
  - [ ] **`/dialog`**
  - [ ] **`/difficulty`**
  - [ ] **`/effect`**
  - [ ] **`/enchant`**
  - [ ] **`/execute`**
  - [ ] **`/experience` and `/xp`**
  - [ ] **`/fill` and `/fillbiome`**
  - [ ] **`/forceload`**
  - [ ] **`/gamemode`**
  - [ ] **`/gamerule`**
  - [ ] **`/give`**
  - [ ] **`/help`**
  - [ ] **`/item`**
  - [ ] **`/kick`**
  - [ ] **`/kill`**
  - [ ] **`/list`**
  - [ ] **`/locate`**
  - [ ] **`/loot`**
  - [ ] **`/me`**
  - [ ] **`/msg`, `/tell` and `/w`**
  - [ ] **`/op` and `/deop`**
  - [ ] **`/particle`**
  - [ ] **`/place`**
  - [ ] **`/playsound` and `/stopsound`**
  - [ ] **`/random`**
  - [ ] **`/recipe`**
  - [ ] **`/reload`**
  - [ ] **`/return`**
  - [ ] **`/ride`**
  - [ ] **`/rotate`**
  - [ ] **`/save-all`, `/save-off` and `/save-on`**
  - [ ] **`/say`**
  - [ ] **`/schedule`**
  - [ ] **`/scoreboard`**
  - [ ] **`/seed`**
  - [ ] **`/setblock`**
  - [ ] **`/setidletimeout`**
  - [ ] **`/setworldspawn`**
  - [ ] **`/spawnpoint`**
  - [ ] **`/spectate`**
  - [ ] **`/spreadplayers`**
  - [ ] **`/stop`**
  - [ ] **`/summon`**
  - [ ] **`/tag`**
  - [ ] **`/team` and `/teammsg`**
  - [ ] **`/teleport` and `/tp`**
  - [ ] **`/tellraw`**
  - [ ] **`/test`**
  - [ ] **`/tick`**
  - [ ] **`/time`**
  - [ ] **`/title`**
  - [ ] **`/transfer`**
  - [ ] **`/trigger`**
  - [ ] **`/version`**
  - [ ] **`/waypoint`**
  - [ ] **`/weather`**
  - [ ] **`/whitelist`**
  - [ ] **`/worldborder`**

## Interface

- [ ] **Advancements**: criteria, triggers, rewards and toasts
- [ ] **Boss bars**: bars for bosses, raids and commands
- [ ] **Dialogs**: server-defined dialog screens
- [ ] **Scoreboards**: objectives, scores and display slots
- [ ] **Statistics**: per-player statistics and the statistics screen
- [ ] **Teams**: colors, prefixes, friendly fire, name tag visibility and collision
- [ ] **Titles**: titles, subtitles and fade timings
- [ ] **Waypoints**: the locator bar

## Sounds and Particles

- [ ] **Explosion effects**: explosion packets with particles, sounds and knockback
- [ ] **Level events**: block breaking, doors, dispensers and other world events
- [ ] **Particles**: particle packets for effects and `/particle`
- [ ] **Sound effects**: block, entity and item sounds sent to nearby players

## Data Packs

- [ ] **Data-driven content**: custom enchantments, damage types, biomes, dimensions, variants and other registries
- [ ] **Loading data packs**: enabling and reloading packs from the world folder
- [ ] **Loot tables**: block, entity, chest and fishing loot
- [ ] **Predicates and item modifiers**: conditions and functions used by loot and commands
- [ ] **Structure templates**: loading `.nbt` structures
- [x] **Vanilla data**: the client's own vanilla pack is used through known packs

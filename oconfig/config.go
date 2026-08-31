package oconfig

import "maps"

const (
	ConfigVersion          uint64 = 7
	DefaultShutdownMessage        = "§cServer is restarting."
)

type Config struct {
	Version uint64 `json:"version" comment:"The version of the config file. This is used to ensure that the config file is compatible with the current version of KillLime.\nDO NOT MODIFY THIS VALUE."`

	Prefix             string `json:"prefix" comment:"The prefix to be used for KillLime."`
	CommandName        string `json:"command_name" comment:"The name of the command used access anti-cheat commands on the proxy. The default is 'ac' which will make the anti-cheat command '/ac'."`
	CommandDescription string `json:"command_description" comment:"The description of the command used access anti-cheat commands on the proxy."`

	GCPercent    int `json:"gc_percent" comment:"Golang's garbage collection percentage. If set to -1 (the default value), the proxy will only run garbage collection when reaching the memory soft limit.\nWe recommend NOT changing this value unless you know what you're doing."`
	MemThreshold int `json:"mem_threshold" comment:"A soft-limit for how much memory the KillLime proxy should use in megabytes. The default value is 1GB (1024MB).\nIf you are running KillLime on a container, we recommend setting this to roughly ~500MB lower to avoid OOM errors.\nIncrease this as neccessary to reduce garbage collection cycles."`

	LocalAddress  string `json:"local_addr" comment:"The address the proxy listens on for incoming connections. For the most part, just using a colon followed by the port is fine."`
	RemoteAddress string `json:"remote_addr" comment:"The address the proxy first connects to for the remote server."`
	BackupAddress string `json:"backup_addr" comment:"The address the proxy will connect to if the inital connection fails to the remote address."`

	ShutdownMessage string `json:"shutdown_message" comment:"The message players are disconnected with when the proxy is shut down and there is no available reconnect address."`
	ReconnectIP     string `json:"reconnect_ip" comment:"The IP address players connected to the proxy are transferred to in the event of a shutdown.\nIf this option is empty, players will be disconnected instead."`
	ReconnectPort   int    `json:"reconnect_port" comment:"The port players connected to the proxy are transferred to in the event of a shutdown.\nIf this option is empty, players will be disconnected instead."`

	UseLegacyEvents bool `json:"use_legacy_events" comment:"This option signifies wether the proxy should use the legacy event system to allow the remote server to handle punishments/flags.\nThis option is recommended to be set to false as the system will be removed in the future."`

	Resource ResourceOpts `json:"resource_opts" comment:"Options for your resource packs."`
	Network  NetworkOpts  `json:"network_opts" comment:"Options for configuring the network settings for KillLime."`
	Movement MovementOpts `json:"movement_opts" comment:"Options for configuring movement policies and strictness for KillLime."`
	Combat   CombatOpts   `json:"combat_opts" comment:"Options for configuring combat policies and strictness for KillLime."`

	SQL  SQLConfig  `json:"sql" comment:"Options for configuring the SQLite admin store backend."`
	Lua  LuaConfig  `json:"lua" comment:"Options for configuring the Lua scripting engine."`

	Detections map[string]Detection `json:"detections" comment:"The configuration for each detection used by the proxy.\nThe allowed punishment types are:\n- none: No punishment will be applied to the player.\n- kick: The player will be kicked when they reach the maximum amount of violations allowed by the detection.\n- ban: The player will be banned when the maximum amount of violations is reached. A ban provider is required for this option to be applied.\nThe tags that can be applied in the flag message are:\n- {player}: The player's username.\n- {xuid}: The player's XBOX Live ID.\n- {violations}: The amount of violations that have been reached on the detection.\n- {prefix}: The prefix defined in the KillLime configuration."`
}

type SQLConfig struct {
	Enabled bool   `json:"enabled" comment:"Enable SQLite as the admin store backend instead of JSON."`
	Path    string `json:"path" comment:"Path to the SQLite database file. Defaults to killlime_admin.db."`
	Migrate bool   `json:"migrate" comment:"Automatically migrate existing JSON admin data to SQLite on first start."`
}

type LuaConfig struct {
	Enabled bool   `json:"enabled" comment:"Enable the Lua scripting engine."`
	Dir     string `json:"dir" comment:"Directory to load .lua scripts from. Defaults to scripts/."`
}

var (
	DefaultConfig = Config{
		Version: ConfigVersion,

		Prefix: "§l§6K§ei§bl§el§6i§em§eb§7§r »",

		CommandName:        "ac",
		CommandDescription: "The command for anti-cheat functionality.",

		GCPercent:    -1,
		MemThreshold: 1024,

		LocalAddress:  ":19132",
		RemoteAddress: ":20000",

		ShutdownMessage: DefaultShutdownMessage,

		Resource: ResourceOpts{
			ResourceFolder: "resources/",
			RequirePacks:   true,
		},

		Network: NetworkOpts{
			AttemptFixChunks:     false,
			UpgradeChunksToBlobs: false,

			GlobalMovementCutoffThreshold: -1,
			MaxGhostBlockChain:            -1,
			MaxACKTimeout:                 60,
			MaxEntityRewind:               6,
			MaxKnockbackDelay:             10,
			MaxBlockUpdateDelay:           -1,
		},

		Movement: MovementOpts{
			CorrectionThreshold:         0.3,
			PersuasionThreshold:         0.002,
			AcceptClientPosition:        false,
			PositionAcceptanceThreshold: 0.09,
			AcceptClientVelocity:        false,
			VelocityAcceptanceThreshold: 0.03,
			LimitAllVelocity:            true,
			LimitAllVelocityThreshold:   10.0,
		},

		Combat: CombatOpts{
			LeftCPSLimit:  20,
			RightCPSLimit: 20,

			LeftCPSLimitMobile:  16,
			RightCPSLimitMobile: 15,

			MaximumAttackAngle:         85.0,
			EnableClientEntityTracking: true,
			AllowNonMobileTouch:        false,
			AllowSwitchInputMode:       false,

			DisableFullAuthoritative:   false,
			DisableBlockOcclusionCheck: false,
			RawDistanceFallback:        false,
			BBoxExpansion:              0.1,
			MaximumReach:               2.9,
			ReachLeniency:              0,
			LerpSteps:                  10,
			EntitySearchRadius:         6,
		},

		Detections: map[string]Detection{
			"Autoclicker_A": {
				MaxVl:      25.0,
				FlagMsg:    "{prefix} §e{player} §6is clicking too quickly §7[§cx{violations}§7]",
				Punishment: PunishmentTypeKick,
				KickCode:   "Combat",
			},
			"Aim_A": {
				MaxVl:      5.0,
				FlagMsg:    "{prefix} §e{player} §6rotated suspiciously §7[§cx{violations}§7]",
				Punishment: PunishmentTypeKick,
				KickCode:   "Combat",
			},
			"BadPacket_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid packet",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_B": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6tried to attack themselves",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_C": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6tried to break blocks with an invalid packet",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_D": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6executed creative action in survival",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_E": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid movement",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket2",
			},
			"BadPacket_F": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid inventory action",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_G": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid interaction",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_H": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid action",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_I": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent a transaction with an invalid slot",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_J": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid creative action",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_K": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid place block action",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket",
			},
			"BadPacket_L": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6failed a position sanity check",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket2",
			},
			"BadPacket_M": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent an invalid jump state",
				Punishment: PunishmentTypeBan,
				KickCode:   "BadPacket2",
			},
			"EditionFaker_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6attempted to spoof their device information",
				Punishment: PunishmentTypeBan,
				KickCode:   "Abagnale",
			},
			"EditionFaker_B": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6attempted to spoof their device information",
				Punishment: PunishmentTypeBan,
				KickCode:   "Abagnale",
			},
			"EditionFaker_C": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6attempted to spoof their device information",
				Punishment: PunishmentTypeBan,
				KickCode:   "Abagnale",
			},
			"Proxy_A": {
				MaxVl:      10.0,
				FlagMsg:    "{prefix} §e{player} §6is likely using a game proxy",
				Punishment: PunishmentTypeKick,
				KickCode:   "Proxy",
			},
			"Proxy_B": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6is connected with a game proxy",
				Punishment: PunishmentTypeBan,
				KickCode:   "Proxy",
			},
			"Hitbox_A": {
				MaxVl:      20.0,
				FlagMsg:    "{prefix} §e{player} §7| §6Hitbox §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
				KickCode:   "Combat",
			},
			"InvMove_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6moving whilst in inventory",
				Punishment: PunishmentTypeBan,
				KickCode:   "FakeLag",
			},
			"Killaura_A": {
				MaxVl:      5.0,
				FlagMsg:    "{prefix} §e{player} §7| §6Killaura §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
				KickCode:   "Combat2",
			},
			"Nuker_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6tried to break blocks using an invalid packet §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
				KickCode:   "Nuker",
			},
			"NoSlowdown_A": {
				MaxVl:      10.0,
				FlagMsg:    "{prefix} §e{player} §6is moving at full speed while using an item §7[§cx{violations}§7]",
				Punishment: PunishmentTypeKick,
				KickCode:   "MovA",
			},
			"Blink_A": {
				MaxVl:      5.0,
				FlagMsg:    "{prefix} §e{player} §6is using blink §7[§cx{violations}§7]",
				Punishment: PunishmentTypeKick,
				KickCode:   "FakeLag",
			},
			"Fly_A": {
				MaxVl:      5.0,
				FlagMsg:    "{prefix} §e{player} §6is flying without permission §7[§cx{violations}§7]",
				Punishment: PunishmentTypeKick,
				KickCode:   "MovA",
			},
			"Speed_B": {
				MaxVl:      10.0,
				FlagMsg:    "{prefix} §e{player} §6is moving too fast in air §7[§cx{violations}§7]",
				Punishment: PunishmentTypeKick,
				KickCode:   "MovA",
			},
			"Reach_A": {
				MaxVl:      10.0,
				FlagMsg:    "{prefix} §e{player} §7| §6Reach §8(Raycast) §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
				KickCode:   "Combat",
			},
			"Reach_B": {
				MaxVl:      30.0,
				FlagMsg:    "{prefix} §e{player} §7| §6Reach §8(Raw) §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
				KickCode:   "Combat",
			},
			"Scaffold_A": {
				MaxVl:      1.0,
				FlagMsg:    "{prefix} §e{player} §6sent invalid action to place blocks §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
				KickCode:   "Scaffold",
			},
			"Scaffold_B": {
				MaxVl:      25.0,
				FlagMsg:    "{prefix} §e{player} §6is placing with an invalid direction §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
				KickCode:   "Scaffold",
			},

			// Cloud detections - max violations are ignored by default and is managed by the cloud instance itself.
			"Cloud_Scaffold": {
				FlagMsg:    "{prefix} §e{player} §6is building suspiciously §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Cloud_Combat": {
				FlagMsg:    "{prefix} §e{player} §6is fighting suspiciously §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Cloud_Aim": {
				FlagMsg:    "{prefix} §e{player} §6is aiming suspiciously §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
			"Cloud_Proxy": {
				FlagMsg:    "{prefix} §e{player} §6is using a game proxy §7[§cx{violations}§7]",
				Punishment: PunishmentTypeBan,
			},
		},
	}
	Global = cloneConfig(DefaultConfig)
)

func cloneConfig(cfg Config) Config {
	cfg.Detections = maps.Clone(cfg.Detections)
	return cfg
}

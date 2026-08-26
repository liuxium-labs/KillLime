-- KillLime Lua Example Script
-- Available APIs: timer, log, register_command, kick_player, ban_player,
--   tempban_player, unban_player, send_message, send_popup, broadcast,
--   get_online_players, is_banned, is_op

print("[KillLime] Script loaded!")

local join_count = 0

function on_join(player)
    join_count = join_count + 1
    log.info(string.format("[Lua] %s joined (xuid=%s, v=%d)", player.name, player.xuid, player.version))
end

function on_quit(player)
    log.info(string.format("[Lua] %s left", player.name))
end

function on_flag(player)
    log.warn(string.format("[Lua] Flag: %s (%s)", player.display_name, player.xuid))
end

function on_reload()
    log.info("[Lua] Scripts reloaded!")
end

-- Register a custom /ac lua command
register_command("lua", "Execute a Lua expression (stub)", "lua <code>", 0, function(sender, args)
    sender.message("§aLua command received!")
    if #args > 0 then
        sender.message("§7Args: " .. table.concat(args, " "))
    end
end)

-- Register a fun command
register_command("online", "Show online player count via Lua", "online", 0, function(sender, args)
    local players = get_online_players()
    sender.message(string.format("§a%d players online", #players))
end)

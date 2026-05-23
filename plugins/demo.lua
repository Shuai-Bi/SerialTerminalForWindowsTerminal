-- Demo Lua plugin for the runtime plugin system.
-- It is shipped disabled by default and only runs after `.plugin load`.

function OnInput(payload)
  return payload
end

function OnOutput(payload)
  return payload
end

function OnCommand(line)
  return line, true
end

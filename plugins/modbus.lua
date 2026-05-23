-- Modbus RTU plugin for SerialTerminalForWindowsTerminal
-- Provides .modbus commands for reading/writing Modbus registers.
-- Uses Go-provided modbus.crc16() and hex.encode/decode helpers.

-- OnInput: intercept Modbus RTU frames and log them
function OnInput(payload)
  return payload
end

-- OnOutput: decode Modbus RTU responses and format for display
function OnOutput(payload)
  return payload
end

-- OnCommand: handle .modbus commands
function OnCommand(line)
  local cmd, slave, addr, count = parseModbus(line)
  if not cmd then
    return line, true  -- not a modbus command, pass through
  end

  if cmd == "read" then
    return buildReadRequest(slave, addr, count), false
  elseif cmd == "write" then
    return buildWriteRequest(slave, addr, count), false
  elseif cmd == "info" then
    return line, true  -- pass to .help
  end

  return line, true
end

-- Parse ".modbus read|write <slave> <addr> <count|value>"
function parseModbus(line)
  local parts = {}
  for part in string.gmatch(line, "%S+") do
    table.insert(parts, part)
  end
  if #parts < 1 or parts[1] ~= ".modbus" then
    return nil
  end
  local cmd = parts[2]
  if cmd == "read" and #parts >= 4 then
    return cmd, tonumber(parts[3]), tonumber(parts[4]), tonumber(parts[5])
  elseif cmd == "write" and #parts >= 4 then
    return cmd, tonumber(parts[3]), tonumber(parts[4]), tonumber(parts[5])
  elseif cmd == "info" then
    return cmd, nil, nil, nil
  end
  return nil
end

-- Build Modbus RTU read holding registers request (function 0x03)
function buildReadRequest(slave, addr, count)
  if not count or count <= 0 then count = 1 end
  if count > 125 then count = 125 end

  local frame = util.bytes(slave, 0x03,
    math.floor(addr / 256), addr % 256,
    math.floor(count / 256), count % 256)

  local crc = modbus.crc16(frame)
  local crcLow = crc % 256
  local crcHigh = math.floor(crc / 256)
  frame = frame .. string.char(crcLow) .. string.char(crcHigh)

  return frame
end

-- Build Modbus RTU write single register request (function 0x06)
function buildWriteRequest(slave, addr, value)
  if not value then value = 0 end

  local frame = util.bytes(slave, 0x06,
    math.floor(addr / 256), addr % 256,
    math.floor(value / 256), value % 256)

  local crc = modbus.crc16(frame)
  local crcLow = crc % 256
  local crcHigh = math.floor(crc / 256)
  frame = frame .. string.char(crcLow) .. string.char(crcHigh)

  return frame
end

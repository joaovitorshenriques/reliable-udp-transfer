-- Load with Wireshark -X lua_script:scripts/rc2.lua
local p = Proto("rc2", "Reliable UDP RC2")
local kinds = {[1]="START",[2]="DATA",[3]="ACK",[4]="FINISH",[5]="DONE",[6]="FAILURE",[7]="REQUEST",[8]="RESPONSE"}
local f = {kind=ProtoField.uint8("rc2.kind","Type",base.DEC,kinds), session=ProtoField.uint64("rc2.session","Session",base.HEX), seq=ProtoField.uint32("rc2.seq","Sequence",base.DEC), window=ProtoField.uint16("rc2.window","Window",base.DEC), length=ProtoField.uint16("rc2.length","Payload length",base.DEC), crc=ProtoField.uint32("rc2.crc","CRC32 (stored, not verified)",base.HEX)}
p.fields=f
p:register_heuristic("udp",function(buf,info,tree)
 if buf:len()<28 or buf(0,4):string()~="RC2U" or buf(4,1):uint()~=1 then return false end
 local kind=buf(5,1):uint()
 if not kinds[kind] or buf:len()~=28+buf(22,2):uint() then return false end
 info.cols.protocol="RC2"
 info.cols.info=kinds[kind].." seq="..buf(16,4):uint().." session="..tostring(buf(8,8):uint64())
 local t=tree:add(p,buf())
 t:add(f.kind,buf(5,1)); t:add(f.session,buf(8,8)); t:add(f.seq,buf(16,4)); t:add(f.window,buf(20,2)); t:add(f.length,buf(22,2)); t:add(f.crc,buf(24,4))
 return true
end)

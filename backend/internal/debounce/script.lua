-- Debounce: atomic "join or create" decision for one component.
--
-- Verbatim from 01-architecture.md §5.3. Loaded once at startup with
-- SCRIPT LOAD; subsequent calls use EVALSHA. Redis executes Lua scripts
-- single-threaded server-side, so the entire check-then-act is atomic
-- across all clients — eliminates the race condition between multiple
-- ingestion workers without a distributed lock.
--
-- KEYS[1] = debounce:{component_id}:work_item
-- KEYS[2] = debounce:{component_id}:count
-- ARGV[1] = candidate_work_item_id (used only if a new window opens)
-- ARGV[2] = window_seconds (10)
-- ARGV[3] = max_signals (100)
--
-- Returns {work_item_id, action, count} where action ∈ {'JOINED', 'CREATED'}.

local existing = redis.call('GET', KEYS[1])
local count = tonumber(redis.call('GET', KEYS[2]) or '0')

if existing and count < tonumber(ARGV[3]) then
    redis.call('INCR', KEYS[2])
    return {existing, 'JOINED', count + 1}
else
    redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
    redis.call('SET', KEYS[2], '1', 'EX', ARGV[2])
    return {ARGV[1], 'CREATED', 1}
end

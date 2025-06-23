package store

const (
	// Script to append messages with metadata updates and optional trim
	// KEYS[1]: the list key
	// KEYS[2]: the metadata hash key
	// ARGV[1]: max length of the list (0 means no trim)
	// ARGV[2]: token count increment
	// ARGV[3...]: messages to append (JSON marshaled)
	// Returns: current length of the list after operation.
	AppendAndTrimWithMetadataScript = `
		local list_key = KEYS[1]
		local meta_key = KEYS[2]
		local max_len = tonumber(ARGV[1])
		local token_incr = tonumber(ARGV[2])
		local msg_count = 0

		-- Append messages
		for i = 3, #ARGV do
			redis.call('RPUSH', list_key, ARGV[i])
			msg_count = msg_count + 1
		end

		-- Update metadata
		if token_incr > 0 then
			redis.call('HINCRBY', meta_key, 'token_count', token_incr)
		end
		redis.call('HINCRBY', meta_key, 'message_count', msg_count)

		-- Trim if needed
		if max_len > 0 then
			redis.call('LTRIM', list_key, -max_len, -1)
			-- Update message count after trim
			local new_len = redis.call('LLEN', list_key)
			redis.call('HSET', meta_key, 'message_count', new_len)
		end

		return redis.call('LLEN', list_key)
	`

	// Script to replace all messages with metadata reset
	// KEYS[1]: the list key
	// KEYS[2]: the metadata hash key
	// ARGV[1]: TTL in milliseconds
	// ARGV[2]: new total token count
	// ARGV[3...]: messages to append (JSON marshaled)
	// Returns: "OK"
	ReplaceMessagesWithMetadataScript = `
		local list_key = KEYS[1]
		local meta_key = KEYS[2]
		local ttl_ms = tonumber(ARGV[1])
		local new_token_count = tonumber(ARGV[2])
		local msg_count = 0

		-- Delete existing list
		redis.call('DEL', list_key)

		-- Add new messages
		for i = 3, #ARGV do
			redis.call('RPUSH', list_key, ARGV[i])
			msg_count = msg_count + 1
		end

		-- Update metadata
		redis.call('HSET', meta_key, 'token_count', new_token_count)
		redis.call('HSET', meta_key, 'message_count', msg_count)

		-- Set TTL if needed
		if ttl_ms > 0 then
			redis.call('PEXPIRE', list_key, ttl_ms)
			redis.call('PEXPIRE', meta_key, ttl_ms)
		end

		return "OK"
	`

	// Script to trim messages and update metadata
	// KEYS[1]: the list key
	// KEYS[2]: the metadata hash key
	// ARGV[1]: number of messages to keep
	// ARGV[2]: new token count after recalculation
	// Returns: new message count
	TrimMessagesWithMetadataScript = `
		local list_key = KEYS[1]
		local meta_key = KEYS[2]
		local keep_count = tonumber(ARGV[1])
		local new_token_count = tonumber(ARGV[2])

		-- Trim list to keep only the last 'keep_count' messages
		redis.call('LTRIM', list_key, -keep_count, -1)
		
		-- Update metadata with actual count and new token count
		local new_len = redis.call('LLEN', list_key)
		redis.call('HSET', meta_key, 'message_count', new_len)
		redis.call('HSET', meta_key, 'token_count', new_token_count)
		
		return new_len
	`

	// Script to get metadata or initialize if missing
	// KEYS[1]: the metadata hash key
	// KEYS[2]: the list key (for counting if metadata missing)
	// Returns: array with [token_count, message_count]
	GetOrInitMetadataScript = `
		local meta_key = KEYS[1]
		local list_key = KEYS[2]
		
		-- Check if metadata exists
		local token_count = redis.call('HGET', meta_key, 'token_count')
		local message_count = redis.call('HGET', meta_key, 'message_count')
		
		if not token_count then
			-- Initialize with actual list length
			local actual_count = redis.call('LLEN', list_key)
			redis.call('HSET', meta_key, 'token_count', 0)
			redis.call('HSET', meta_key, 'message_count', actual_count)
			return {0, actual_count}
		end
		
		return {token_count, message_count}
	`
)

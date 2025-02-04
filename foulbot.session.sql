SELECT user_id, COUNT(*) as appearance_count
FROM gainers
GROUP BY user_id
ORDER BY appearance_count DESC;
SELECT COALESCE(SUM(p.points), 0) AS total_points
FROM polls p
    JOIN gainers g ON p.message_id = g.message_id
WHERE g.user_id = ?
    AND strftime('%Y', p.expiry) = ?
    AND p.passed = 1;
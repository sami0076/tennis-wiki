-- name: Health :one
-- Touches a real table, so a database with no schema is distinguishable from
-- a healthy one rather than both merely having an open socket.
SELECT EXISTS (SELECT 1 FROM players) AS ready;

-- PostgreSQL table creation script for user submission tracking
-- This creates the submissions table with minute-level aggregation

CREATE TABLE IF NOT EXISTS submissions (
    username VARCHAR(255) NOT NULL,
    timestamp TIMESTAMP NOT NULL, -- 精确到分钟
    submission_count INT NOT NULL DEFAULT 1,
    PRIMARY KEY (username, timestamp)
);

-- Create index for efficient time-range queries
CREATE INDEX IF NOT EXISTS idx_submissions_timestamp ON submissions (timestamp);
CREATE INDEX IF NOT EXISTS idx_submissions_username ON submissions (username);

-- Additional tables for StateStore functionality
CREATE TABLE IF NOT EXISTS job_history (
    job_id INT PRIMARY KEY,
    server_nonce VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS user_state (
    username VARCHAR(255) PRIMARY KEY,
    latest_job_id INT NOT NULL,
    latest_server_nonce VARCHAR(64),
    last_submit_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS used_nonce (
    username VARCHAR(255) NOT NULL,
    job_id INT NOT NULL,
    client_nonce VARCHAR(64) NOT NULL,
    PRIMARY KEY (username, job_id, client_nonce)
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_job_history_created_at ON job_history (created_at);
CREATE INDEX IF NOT EXISTS idx_user_state_username ON user_state (username);
CREATE INDEX IF NOT EXISTS idx_used_nonce_username ON used_nonce (username);
CREATE INDEX IF NOT EXISTS idx_used_nonce_job_id ON used_nonce (job_id);

-- Example usage queries:

-- 1. Insert or update submission count (upsert)
INSERT INTO submissions (username, timestamp, submission_count) 
VALUES ('user123', '2024-01-15 14:30:00', 1)
ON CONFLICT (username, timestamp) 
DO UPDATE SET submission_count = submissions.submission_count + 1;

-- 2. Get submission count for a specific user and minute
SELECT submission_count 
FROM submissions 
WHERE username = 'user123' 
AND timestamp = '2024-01-15 14:30:00';

-- 3. Get user submissions within a time range
SELECT timestamp, submission_count 
FROM submissions 
WHERE username = 'user123' 
AND timestamp >= '2024-01-15 14:00:00' 
AND timestamp <= '2024-01-15 15:00:00'
ORDER BY timestamp ASC;

-- 4. Get top users by submissions in a time range
SELECT username, SUM(submission_count) as total_submissions
FROM submissions 
WHERE timestamp >= '2024-01-15 14:00:00' 
AND timestamp <= '2024-01-15 15:00:00'
GROUP BY username
ORDER BY total_submissions DESC
LIMIT 10;

-- 5. Get total submissions across all users in a time range
SELECT SUM(submission_count) as total_submissions
FROM submissions 
WHERE timestamp >= '2024-01-15 14:00:00' 
AND timestamp <= '2024-01-15 15:00:00';
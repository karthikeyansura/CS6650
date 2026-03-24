## Assignment: Debug the Hummingbird API using Claude Code
 
### Overview
 
In this assignment you'll step into the role of a new engineer on a real API team. The API is already deployed on AWS — but it has bugs. Your job is to find them, fix them, and explain what went wrong.
 
You'll use **Claude Code** (an AI coding assistant that runs in your terminal) to investigate the live system, read the source code, and trace errors in CloudWatch logs. No prior AWS experience is required — everything you need is set up for you.
 
---
 
### What You're Working With (thank you Mansi!): 
 
**Hummingbird** (https://github.com/modimansi/humming-bird-midterm) is a media management REST API built with Node.js, Express, AWS S3, DynamoDB, and SNS. It lets users upload images, track processing status, and download a processed version.
 
The codebase has **real bugs planted across multiple layers**. Your job is to find and fix them.
 
---
 
### Before You Start — Prerequisites: 
 
Make sure you have:
- [ ] Access to the AWS account provided by your instructor
- [ ] The Hummingbird repository cloned in your AWS environment
- [ ] Claude Code installed and configured (see setup steps below)
 
---
 
### Step 1 — Set Up Claude Code in AWS CloudShell
 
1. Open **AWS CloudShell** from the AWS Console (the `>_` icon in the top navigation bar)
 
2. Install Claude Code 
 
3. Configure Claude Code to use **AWS Bedrock** (no separate API key needed — it uses your AWS credentials):
https://code.claude.com/docs/en/amazon-bedrock
 
4. In your CloudShell terminal, run these two commands:
   ```
     export CLAUDE_CODE_USE_BEDROCK=1
     export AWS_REGION=<your-aws-region>
     export ANTHROPIC_MODEL='us.anthropic.claude-opus-4-6-v1'
   ```
 
5. Start Claude Code:
   ```
   claude
   ```
 
---
 
### Step 2 — Read the README
 
Navigate to the Hummingbird repo and open **`README.md`**.
 
This file explains:
- What the API does
- How the infrastructure is deployed (Terraform + ECS + ALB)
- How to make API calls using `curl`
- How to view live logs in CloudWatch
 
Read through it before doing anything else. The API URL for your environment will be provided by your instructor.
 
---
 
### Step 3 — Complete the Assignment
 
Open **`ASSIGNMENT.md`** in the repo. This is your task sheet.
 
It contains **4 support tickets** (2 easy, 2 intermediate) plus **1 bonus challenge**. Each ticket describes a real bug a user reported. For each one you will:
 
1. **Reproduce** the bug using `curl` against the live API
2. **Investigate** using Claude Code — ask it to trace the symptom through the source code and CloudWatch logs
3. **Fix** the source code
4. **Redeploy** using the `docker build` + `aws ecs update-service` commands in the assignment
5. **Verify** the fix works by hitting the API again
 
---
 
### What to Submit
 
Upload a document (PDF or Google Doc) containing:
 
| Section | What to include |
|---|---|
| **For each ticket** | The bug you found, the exact file and line, and a brief explanation of why it was broken |
| **Code diff** | Copy-paste the before/after of each fix (ask Claude Code: *"show me a diff of that change"*) |
| **Your jsonl file** | Just upload this file, raw, do no copy and past it into a PDF |
| **Bonus (optional)** | Your investigation notes for the bonus challenge |
 
---
 
### Tips
 
- **Start with Claude Code's codebase walkthrough** before touching any bug — it's the first instruction in `ASSIGNMENT.md` for a reason
- **CloudWatch logs are your best friend** — every ticket has a log line that gives you a clue. Use `aws logs tail /ecs/hummingbird/production/api --follow` to watch them in real time
- **Ask Claude Code to explain, not just fix** — if it finds the bug, ask *why* it's a bug before asking it to apply the fix. You need to understand it for the reflection questions
- **The bonus ticket has zero errors in the logs** — that's the point. You'll need to read multiple log lines side by side to spot it
 
---
 
### Grading
 
| Component | Weight |
|---|---|
| All 4 tickets found and fixed with correct explanation | 10 points |
| Bonus challenge | 2 points (extra credit) |
 

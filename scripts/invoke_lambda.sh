aws lambda invoke \
  --function-name lambda_techdetector \
  --region eu-west-2 \
  --cli-binary-format raw-in-base64-out \
  --payload '{
    "body": "{\"repo\": \"https://github.com/reaandrew/techdetector.git\"}"
  }' \
  output.json
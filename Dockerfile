dockerfile

# Use Amazon Linux 2 as the base image (matches Lambda runtime)
FROM public.ecr.aws/lambda/provided:al2

# Set working directory
WORKDIR /var/task

# Copy the pre-built binary from the artifact
COPY techdetector-linux-amd64 bootstrap

# Copy additional files (e.g., queries.yaml)
COPY queries.yaml .

# Set the Lambda handler
CMD ["bootstrap"]
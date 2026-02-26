# Node.js Memory Stress Test Application
# Compatible with Azure App Service (Linux) and Azure Container Apps

FROM node:20-slim

# Set environment variables
ENV NODE_ENV=production \
    PORT=8080

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm install --omit=dev && \
    npm cache clean --force

# Copy application code
COPY app.js .

# Create non-root user for security
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nodeuser && \
    chown -R nodeuser:nodejs /app

USER nodeuser

# Expose port
EXPOSE 8080

# Health check for container orchestration
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD node -e "require('http').get('http://localhost:8080/', (r) => process.exit(r.statusCode === 200 ? 0 : 1))" || exit 1

# Run the application
# --expose-gc allows the /reset endpoint to force garbage collection
# Node.js 20 auto-detects container memory limits via cgroup
CMD ["node", "--expose-gc", "app.js"]

import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  output: 'standalone', // Giúp giảm kích thước build khi deploy
  images: {
    unoptimized: true // Nếu không dùng Next Image Optimization hoặc dùng server riêng
  }
}

export default nextConfig

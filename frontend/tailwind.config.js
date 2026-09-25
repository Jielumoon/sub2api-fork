// 画布与辅助色共用暖中性色，避免把表面染成主色。
const warmGray = {
  50: '#fafaf7',
  100: '#f4f2ed',
  200: '#e9e6de',
  300: '#d7d4cc',
  400: '#b0aea6',
  500: '#76726d',
  600: '#5d5a56',
  700: '#474441',
  800: '#2d2b28',
  900: '#201e1c',
  950: '#151312'
}

/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 陶土主色；白字按钮使用 600，而不是对比度不足的 500。
        primary: {
          50: '#fff4f0',
          100: '#fee7df',
          200: '#fecfbe',
          300: '#faaf96',
          400: '#f19173',
          500: '#d97757',
          600: '#b55336',
          700: '#97442d',
          800: '#793626',
          900: '#602b1e',
          950: '#38170e'
        },
        gray: warmGray,
        accent: warmGray,
        // 保留页面、卡片、描边的既有色阶语义。
        dark: {
          50: '#f9f8f5',
          100: '#f2f0ec',
          200: '#e0ded8',
          300: '#c0bdb8',
          400: '#9a9893',
          500: '#777471',
          600: '#575552',
          700: '#3f3d3a',
          800: '#2c2a28',
          900: '#22201f',
          950: '#1b1918'
        }
      },
      fontFamily: {
        sans: [
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        // 不前置 ui-serif，避免中文被系统衬线字体提前接管。
        serif: ['Georgia', 'Cambria', 'Times New Roman', 'PingFang SC', 'Microsoft YaHei', 'sans-serif'],
        // 中文补在通用 monospace 之前，否则 Windows 上等宽文本里的中文会退到宋体。
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'PingFang SC', 'Microsoft YaHei', 'monospace']
      },
      boxShadow: {
        glass: '0 8px 32px rgba(0, 0, 0, 0.08)',
        'glass-sm': '0 4px 16px rgba(0, 0, 0, 0.06)',
        glow: '0 0 20px rgba(217, 119, 87, 0.08)',
        'glow-lg': '0 0 40px rgba(217, 119, 87, 0.12)',
        card: '0 1px 2px rgba(32, 30, 28, 0.03)',
        'card-hover': '0 12px 28px -14px rgba(32, 30, 28, 0.22)',
        // 主按钮悬停时的暖色投影（primary-700）。
        lift: '0 6px 16px -6px rgba(151, 68, 45, 0.45)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, #d97757 0%, #b55336 100%)',
        'gradient-dark': 'linear-gradient(135deg, #2c2a28 0%, #22201f 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient':
          'radial-gradient(at 70% 10%, rgba(217, 119, 87, 0.12) 0px, transparent 55%), radial-gradient(at 12% 85%, rgba(120, 140, 93, 0.08) 0px, transparent 50%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 20px rgba(217, 119, 87, 0.08)' },
          '100%': { boxShadow: '0 0 30px rgba(217, 119, 87, 0.12)' }
        }
      },
      // 带轻微回弹的缓动，用于弹窗和 Toast 入场。
      transitionTimingFunction: {
        spring: 'cubic-bezier(0.34, 1.4, 0.64, 1)'
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}

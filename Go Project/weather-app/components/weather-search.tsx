'use client'

import { useState } from 'react'
import { Search } from 'lucide-react'
import { Sun, Cloud, CloudRain, CloudDrizzle, CloudLightning, CloudFog } from 'lucide-react'

export default function WeatherSearch() {
  const [city, setCity] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [weatherData, setWeatherData] = useState<any>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    setWeatherData(null)
  
    try {
      // Sadece bu kısmı güncelliyoruz
      const response = await fetch('/api/weather', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ city }),
      })
  
      const result = await response.json()
  
      if (!result.success) {
        throw new Error(result.message)
      }
  
      setWeatherData(result.data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Bir hata oluştu')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex flex-col items-center pt-20 px-4">
      {/* Weather Icons */}
      <div className="flex gap-6 mb-8">
        <Sun className="w-8 h-8 text-yellow-400" />
        <Cloud className="w-8 h-8 text-gray-400" />
        <CloudRain className="w-8 h-8 text-blue-400" />
        <CloudDrizzle className="w-8 h-8 text-blue-300" />
        <CloudLightning className="w-8 h-8 text-yellow-500" />
        <CloudFog className="w-8 h-8 text-blue-300" />
      </div>

      {/* Search Form */}
      <form onSubmit={handleSubmit} className="w-full max-w-2xl">
        <div className="relative">
          <input
            type="text"
            value={city}
            onChange={(e) => setCity(e.target.value)}
            placeholder="Hava durumunu öğrenmek istediğiniz şehri giriniz"
            className="w-full px-4 py-3 pr-12 rounded-full border border-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent text-black"
            disabled={loading}
          />
          <button
            type="submit"
            disabled={loading}
            className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
          >
            <Search className="w-5 h-5" />
          </button>
        </div>
      </form>

      {/* Loading Message */}
      {loading && <div className="mt-4 text-blue-500">Yükleniyor...</div>}

      {/* Error Message */}
      {error && <div className="mt-4 text-red-500">{error}</div>}

      {/* Weather Data */}
      {weatherData && (
        <div className="mt-8 p-6 bg-white rounded-lg shadow-md text-black">
          <h2 className="text-2xl font-bold mb-4">{weatherData.city} Hava Durumu</h2>
          <p className="text-lg">Sıcaklık: {weatherData.temperature}°C</p>
          <p className="text-lg">Durum: {weatherData.condition}</p>
        </div>
      )}
    </div>
  )
}


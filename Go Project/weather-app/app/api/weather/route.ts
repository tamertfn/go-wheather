export async function POST(request: Request) {
  try {
    const { city } = await request.json()
    
    // Golang backend'e GET isteği at
    const response = await fetch(`http://localhost:8080/api/v1/weather/cities/${encodeURIComponent(city)}`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      }
    })

    if (!response.ok) {
      const errorData = await response.json()
      throw new Error(errorData.error || 'Backend request failed')
    }

    const data = await response.json()
    
    return Response.json({
      success: true,
      data: data
    })
  } catch (error) {
    console.error('Error:', error)
    return Response.json(
      { success: false, message: "Hava durumu verileri alınamadı" },
      { status: 500 }
    )
  }
}
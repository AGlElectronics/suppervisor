import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'

describe('Suppervisor PWA', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
    localStorage.clear()
  })

  it('keeps the complete dinner workflow usable with labelled demo data', async () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'What should dinner do for you?' })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText(/Previewing seeded demo data/)).toBeInTheDocument())

    fireEvent.click(screen.getAllByRole('button', { name: 'Recipe' })[0])
    expect(screen.getByRole('heading', { name: 'One-pan tomato lentil orzo' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Open shopping/ }))
    expect(screen.getByText(/0 of 5 tucked into the basket/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('checkbox', { name: /orzo/ }))
    expect(screen.getByText(/1 of 5 tucked into the basket/)).toBeInTheDocument()
  })

  it('shows distinct diner profiles and management sections', async () => {
    render(<App />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Profile' })[0])
    expect(screen.getByRole('heading', { name: 'Alan' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Partner' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Manage' }))
    expect(screen.getByRole('tab', { name: 'preferences' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'corrections' })).toBeInTheDocument()
  })
})

import { describe, it, expect } from 'vitest';
import { isAllowedWallpaperUrl } from './constants';

describe('isAllowedWallpaperUrl', () => {
  // Allowed: imgur.com subdomains and exact
  it('allows imgur.com URLs', () => {
    expect(isAllowedWallpaperUrl('https://i.imgur.com/photo.png')).toBe(true);
  });

  it('allows unsplash.com URLs', () => {
    expect(isAllowedWallpaperUrl('https://images.unsplash.com/photo.png')).toBe(true);
  });

  it('allows picsum.photos URLs', () => {
    expect(isAllowedWallpaperUrl('https://picsum.photos/id/237/200/300')).toBe(true);
  });

  // Allowed: local paths
  it('allows local / paths', () => {
    expect(isAllowedWallpaperUrl('/wallpapers/bg.png')).toBe(true);
    expect(isAllowedWallpaperUrl('/images/custom.jpg')).toBe(true);
  });

  // Rejected: external malicious domains
  it('rejects evil.com URLs', () => {
    expect(isAllowedWallpaperUrl('http://evil.com/evil.png')).toBe(false);
  });

  it('rejects arbitrary external domains', () => {
    expect(isAllowedWallpaperUrl('https://example.com/image.png')).toBe(false);
    expect(isAllowedWallpaperUrl('http://google.com/image.png')).toBe(false);
  });

  // Rejected: relative paths (no leading /)
  it('rejects relative paths', () => {
    expect(isAllowedWallpaperUrl('images/bg.png')).toBe(false);
    expect(isAllowedWallpaperUrl('./wallpaper.jpg')).toBe(false);
  });

  // Rejected: invalid URLs
  it('rejects non-http protocols', () => {
    expect(isAllowedWallpaperUrl('file:///etc/passwd')).toBe(false);
    expect(isAllowedWallpaperUrl('javascript:alert(1)')).toBe(false);
  });

  it('rejects empty string', () => {
    expect(isAllowedWallpaperUrl('')).toBe(false);
  });
});

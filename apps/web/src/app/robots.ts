import { MetadataRoute } from 'next';

export default function robots(): MetadataRoute.Robots {
  const baseUrl = 'https://1herosocial.ai';
  return {
    rules: {
      userAgent: '*',
      allow: ['/', '/pricing', '/about', '/login'],
      disallow: ['/admin/', '/api/', '/dashboard/'],
    },
    sitemap: `${baseUrl}/sitemap.xml`,
  };
}

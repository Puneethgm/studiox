import { redirect } from 'next/navigation';

// Root redirects to /admin — the auth gate there handles the rest.
export default function Home() {
  redirect('/admin');
}

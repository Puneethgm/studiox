'use client';

export default function TermsAndConditions() {
  return (
    <main className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-950 dark:to-slate-900 px-4 py-16">
      <div className="max-w-4xl mx-auto">
        <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 p-8 md:p-12 shadow-lg">
          <h1 className="text-4xl font-black mb-2 text-slate-900 dark:text-white">Terms and Conditions</h1>
          <p className="text-sm text-slate-500 mb-8">Last updated: June 2026 | For 1herosocial.ai</p>

          <div className="space-y-8 text-slate-700 dark:text-slate-300">
            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Agreement to Terms</h2>
              <p>By accessing and using 1herosocial.ai, you agree to be bound by these Terms and Conditions. If you do not agree, please do not use this service.</p>
              <p className="mt-4"><strong>Contact:</strong> support@1herosocial.ai</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Use License</h2>
              <p className="mb-3">Permission is granted to temporarily download materials from 1herosocial.ai for personal, non-commercial use. You may NOT:</p>
              <ul className="list-disc pl-6 space-y-1">
                <li>Modify or copy the materials</li>
                <li>Use materials for any commercial purpose</li>
                <li>Attempt to decompile or reverse engineer software</li>
                <li>Transfer materials to another person</li>
                <li>Violate applicable laws or regulations</li>
                <li>Access the Service through automated means</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Warranty Disclaimer</h2>
              <p>The Service is provided "as-is" without warranties. We make no representations about accuracy, reliability, or that the Service will be uninterrupted or error-free.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Limitation of Liability</h2>
              <p>We are not liable for indirect, incidental, special, or consequential damages, including lost profits. Our total liability shall not exceed what you paid us in the 12 months preceding any claim.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">User Accounts</h2>
              <p className="mb-3">If you create an account, you are responsible for:</p>
              <ul className="list-disc pl-6 space-y-1">
                <li>Maintaining password confidentiality</li>
                <li>All activities under your account</li>
                <li>Keeping account information accurate</li>
                <li>Notifying us of unauthorized access</li>
              </ul>
              <p className="mt-4">You must be 18 years or older to use the Service.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Prohibited Conduct</h2>
              <p className="mb-3">You agree not to:</p>
              <ul className="list-disc pl-6 space-y-1">
                <li>Violate any laws or regulations</li>
                <li>Harass, abuse, or threaten other users</li>
                <li>Attempt unauthorized system access</li>
                <li>Use the Service for spam or illegal purposes</li>
                <li>Upload viruses or malicious code</li>
                <li>Collect personal information without consent</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Payment and Billing</h2>
              <ul className="list-disc pl-6 space-y-1">
                <li>You authorize us to charge your payment method</li>
                <li>You are responsible for maintaining accurate payment information</li>
                <li>Subscriptions are non-refundable</li>
                <li>You can cancel anytime; cancellation takes effect at end of billing cycle</li>
                <li>We may change pricing with 30 days notice</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Account Termination</h2>
              <p>We may terminate your account if you violate these Terms, engage in illegal activity, fail to pay fees, or provide false information. Upon termination, your right to use the Service immediately ceases.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Privacy and Data</h2>
              <p className="mb-3">Your use is governed by our Privacy Policy. Key points:</p>
              <ul className="list-disc pl-6 space-y-1">
                <li>We comply with GDPR, CCPA, and data protection laws</li>
                <li>You have rights to access, correct, and delete your data</li>
                <li>Payment records retained 7 years (tax law requirement)</li>
                <li>Audit logs retained 90 days (security requirement)</li>
              </ul>
              <p className="mt-4"><strong>Privacy Policy:</strong> <a href="/privacy" className="text-violet-600 hover:text-violet-700">https://1herosocial.ai/privacy</a></p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Account Deletion</h2>
              <p className="mb-3">You can delete your account permanently at any time:</p>
              <ol className="list-decimal pl-6 space-y-1">
                <li>Go to Settings → Security → Delete Account</li>
                <li>Confirm your email address</li>
                <li>Your account and data will be deleted within 24 hours</li>
              </ol>
              <p className="mt-4 text-sm bg-red-50 border border-red-200 rounded p-4 text-red-800">
                <strong>Warning:</strong> This action is permanent and irreversible. All data will be deleted except payment records (7 years) and audit logs (90 days) as required by law.
              </p>
              <p className="mt-4"><strong>Delete Account:</strong> <a href="/delete-account" className="text-violet-600 hover:text-violet-700">https://1herosocial.ai/delete-account</a></p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Third-Party Services</h2>
              <p className="mb-3">The Service integrates with:</p>
              <ul className="list-disc pl-6 space-y-1">
                <li>Stripe (payment processing)</li>
                <li>Meta/WhatsApp (messaging)</li>
                <li>Facebook Messenger (messaging)</li>
                <li>Instagram (messaging)</li>
                <li>AWS (hosting and storage)</li>
              </ul>
              <p className="mt-4">Each third party has their own terms and privacy policies. We are not responsible for their practices.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Intellectual Property</h2>
              <p>All content, features, and functionality are the exclusive property of 1herosocial.ai and protected by international copyright and intellectual property laws.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Modifications</h2>
              <p>We may revise these Terms at any time without notice. Your continued use of the Service means you accept the updated terms.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Governing Law</h2>
              <p>These terms are governed by applicable law. Disputes shall be resolved according to the laws of the relevant jurisdiction.</p>
            </section>

            <section className="bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 rounded-lg p-6">
              <h3 className="font-bold text-slate-900 dark:text-white mb-3">Important Reminders</h3>
              <ul className="space-y-2 text-sm">
                <li>Account deletion is permanent and irreversible</li>
                <li>Your privacy is important - read our Privacy Policy</li>
                <li>The Service is provided as-is without warranties</li>
                <li>Payment records are retained 7 years for tax compliance</li>
                <li>GDPR and CCPA users have additional rights</li>
                <li>Contact support@1herosocial.ai for legal notices</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Contact</h2>
              <p><strong>Email:</strong> support@1herosocial.ai</p>
            </section>
          </div>
        </div>
      </div>
    </main>
  );
}

'use client';

export default function DeleteAccountPage() {
  return (
    <main className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-950 dark:to-slate-900 px-4 py-16">
      <div className="max-w-4xl mx-auto">
        <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 p-8 md:p-12 shadow-lg">
          <h1 className="text-4xl font-black mb-2 text-slate-900 dark:text-white">Delete Your Account</h1>
          <p className="text-sm text-slate-500 mb-8">Permanent Account Deletion and Data Removal Guide</p>

          <div className="space-y-8 text-slate-700 dark:text-slate-300">
            <section className="bg-red-50 dark:bg-red-950/30 border-l-4 border-red-500 rounded p-6">
              <h2 className="text-2xl font-bold mb-4 text-red-900 dark:text-red-100">Warning</h2>
              <p className="text-red-900 dark:text-red-100">Account deletion is <strong>permanent and irreversible</strong>. Once deleted, you cannot recover your account, messages, or data. This action cannot be undone.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">How to Delete Your Account</h2>
              <p className="mb-4">Follow these steps to permanently delete your account:</p>

              <ol className="space-y-4">
                <li className="flex gap-4">
                  <span className="flex-shrink-0 w-8 h-8 bg-violet-600 text-white rounded-full flex items-center justify-center font-bold">1</span>
                  <div>
                    <h3 className="font-bold mb-1">Log into your account</h3>
                    <p className="text-sm">Go to 1herosocial.ai and log in with your credentials</p>
                  </div>
                </li>

                <li className="flex gap-4">
                  <span className="flex-shrink-0 w-8 h-8 bg-violet-600 text-white rounded-full flex items-center justify-center font-bold">2</span>
                  <div>
                    <h3 className="font-bold mb-1">Go to Settings</h3>
                    <p className="text-sm">Click the Settings icon or navigate to Account Settings</p>
                  </div>
                </li>

                <li className="flex gap-4">
                  <span className="flex-shrink-0 w-8 h-8 bg-violet-600 text-white rounded-full flex items-center justify-center font-bold">3</span>
                  <div>
                    <h3 className="font-bold mb-1">Click the Security tab</h3>
                    <p className="text-sm">Look for the Security section in your settings</p>
                  </div>
                </li>

                <li className="flex gap-4">
                  <span className="flex-shrink-0 w-8 h-8 bg-violet-600 text-white rounded-full flex items-center justify-center font-bold">4</span>
                  <div>
                    <h3 className="font-bold mb-1">Click Delete Account Permanently</h3>
                    <p className="text-sm">Scroll down to the bottom. You will see a red Delete Account button</p>
                  </div>
                </li>

                <li className="flex gap-4">
                  <span className="flex-shrink-0 w-8 h-8 bg-violet-600 text-white rounded-full flex items-center justify-center font-bold">5</span>
                  <div>
                    <h3 className="font-bold mb-1">Enter your email and check the confirmation box</h3>
                    <p className="text-sm">Confirm your account email and tick "I am sure I want to permanently delete this account"</p>
                  </div>
                </li>

                <li className="flex gap-4">
                  <span className="flex-shrink-0 w-8 h-8 bg-violet-600 text-white rounded-full flex items-center justify-center font-bold">6</span>
                  <div>
                    <h3 className="font-bold mb-1">Check your email</h3>
                    <p className="text-sm">We send a confirmation link to your account email. Nothing is deleted yet — open that email and click the link to proceed</p>
                  </div>
                </li>

                <li className="flex gap-4">
                  <span className="flex-shrink-0 w-8 h-8 bg-violet-600 text-white rounded-full flex items-center justify-center font-bold">7</span>
                  <div>
                    <h3 className="font-bold mb-1">Account deleted</h3>
                    <p className="text-sm">Clicking the emailed link permanently deletes your account. A separate email with a 7-day recovery link is sent right after, in case you change your mind</p>
                  </div>
                </li>
              </ol>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">What Gets Deleted</h2>
              <p className="mb-4">When you delete your account, the following data is permanently removed:</p>

              <ul className="space-y-2 mb-6">
                <li className="flex items-start gap-3">
                  <span className="text-green-600 font-bold mt-1">✓</span>
                  <span>Your account profile and login credentials</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-green-600 font-bold mt-1">✓</span>
                  <span>All studio information and settings</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-green-600 font-bold mt-1">✓</span>
                  <span>All messages and conversations</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-green-600 font-bold mt-1">✓</span>
                  <span>All uploaded files and documents</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-green-600 font-bold mt-1">✓</span>
                  <span>API credentials and tokens</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-green-600 font-bold mt-1">✓</span>
                  <span>Payment methods and billing information</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-green-600 font-bold mt-1">✓</span>
                  <span>All personal data we hold about you</span>
                </li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">What Is Retained (Legal Requirements)</h2>
              <p className="mb-4">Due to legal obligations, the following data is retained for the specified periods:</p>

              <ul className="space-y-2">
                <li className="flex items-start gap-3">
                  <span className="text-slate-400 font-bold mt-1">•</span>
                  <span><strong>Payment records:</strong> 7 years (tax compliance and fraud prevention)</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-slate-400 font-bold mt-1">•</span>
                  <span><strong>Audit logs:</strong> 90 days (security and debugging)</span>
                </li>
                <li className="flex items-start gap-3">
                  <span className="text-slate-400 font-bold mt-1">•</span>
                  <span><strong>Deleted data backups:</strong> 7 days (self-service recovery window)</span>
                </li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Timeline</h2>
              <p className="mb-4">Here is what happens after you request account deletion:</p>

              <div className="space-y-3">
                <div className="flex gap-4">
                  <div className="flex-shrink-0 w-32 font-bold text-violet-600">On request</div>
                  <div>We email a confirmation link to your account email. Nothing is deleted yet.</div>
                </div>
                <div className="flex gap-4">
                  <div className="flex-shrink-0 w-32 font-bold text-violet-600">On confirmation</div>
                  <div>Clicking the emailed link runs the deletion immediately: all data, messages, files, and credentials are permanently removed.</div>
                </div>
                <div className="flex gap-4">
                  <div className="flex-shrink-0 w-32 font-bold text-violet-600">Right after</div>
                  <div>A second email is sent with a 7-day recovery link and instructions for downloading an archive of your data.</div>
                </div>
                <div className="flex gap-4">
                  <div className="flex-shrink-0 w-32 font-bold text-violet-600">After 7 days</div>
                  <div>The recovery archive expires and is permanently removed. Deletion is then final.</div>
                </div>
              </div>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">After Deletion</h2>
              <ul className="space-y-3">
                <li>You will receive a confirmation email once deletion is complete, with a separate 7-day recovery link</li>
                <li>You can no longer log in with your credentials</li>
                <li>Your studio name and profile will no longer be visible</li>
                <li>All third-party integrations (Stripe, Meta, etc.) will be disconnected</li>
                <li>You cannot undo or recover your account after the 7-day recovery window closes</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Recovery Window</h2>
              <p className="mb-4">You have 7 days from the moment of deletion to recover your data yourself:</p>
              <ol className="list-decimal pl-6 space-y-1">
                <li>Open the recovery link from the email sent right after deletion</li>
                <li>Re-enter your studio&apos;s registered account email to confirm it&apos;s you</li>
                <li>Download the account archive (a ZIP file) directly from the page</li>
              </ol>
              <p className="mt-4 text-sm text-slate-600 dark:text-slate-400">After 7 days, the recovery link expires and the archive is permanently removed.</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Your Rights</h2>
              <p className="mb-4">Under GDPR and CCPA, you have the right to:</p>
              <ul className="list-disc pl-6 space-y-1">
                <li>Request deletion of your personal data at any time</li>
                <li>Receive a copy of your data before deletion</li>
                <li>Request confirmation that data has been deleted</li>
                <li>File a complaint with your local data protection authority</li>
              </ul>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">Questions?</h2>
              <p>For questions about account deletion or data privacy:</p>
              <p className="mt-4"><strong>Email:</strong> <a href="mailto:1herosocialai@gmail.com" className="text-blue-600 dark:text-blue-400 underline">1herosocialai@gmail.com</a></p>
              <p className="mt-2"><strong>Subject line:</strong> Start with "PRIVACY:" or "ACCOUNT DELETION:"</p>
              <p className="mt-2"><strong>Response time:</strong> 5 business days</p>
            </section>

            <section>
              <h2 className="text-2xl font-bold mb-4 text-slate-900 dark:text-white">More Information</h2>
              <ul className="space-y-2">
                <li><a href="/privacy" className="text-violet-600 hover:text-violet-700 font-semibold">Privacy Policy</a> - How we collect and use your data</li>
                <li><a href="/terms" className="text-violet-600 hover:text-violet-700 font-semibold">Terms of Service</a> - Our full terms and conditions</li>
                <li><strong>Email:</strong> <a href="mailto:1herosocialai@gmail.com" className="text-blue-600 dark:text-blue-400 underline">1herosocialai@gmail.com</a> - Contact us anytime</li>
              </ul>
            </section>
          </div>
        </div>
      </div>
    </main>
  );
}

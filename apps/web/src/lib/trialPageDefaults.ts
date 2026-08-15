import type { PageBlock, PageBackground } from './types';

// Used both by the admin builder (starting point when a studio has never
// customized the page) and the public trial payment page (fallback render
// when the studio has no saved layout) — kept in one place so they can never
// drift apart.
export const CANVAS_WIDTH = 390;
export const CANVAS_MIN_HEIGHT = 600;

export function defaultTrialPageBackground(): PageBackground {
  return { color: '#f9fafb' };
}

export function defaultTrialPageBlocks(): PageBlock[] {
  return [
    {
      id: 'default-heading', type: 'text', x: 20, y: 20, width: 350, height: 40, zIndex: 1,
      content: { text: 'Almost there! 🎉', fontSize: 22, color: '#111827', weight: 'bold', fontFamily: 'system' },
    },
    {
      id: 'default-subheading', type: 'text', x: 20, y: 66, width: 350, height: 50, zIndex: 1,
      content: { text: 'A few quick details, then straight to secure payment.', fontSize: 13, color: '#6b7280', weight: 'normal', fontFamily: 'system' },
    },
    {
      id: 'default-name', type: 'name_field', x: 20, y: 128, width: 350, height: 58, zIndex: 1,
      content: { label: 'Full Name' },
    },
    {
      id: 'default-gender', type: 'gender_field', x: 20, y: 196, width: 350, height: 58, zIndex: 1,
      content: { label: 'Gender (optional)' },
    },
    {
      id: 'default-dob', type: 'dob_field', x: 20, y: 264, width: 350, height: 58, zIndex: 1,
      content: { label: 'Date of Birth (optional)' },
    },
    {
      id: 'default-amount', type: 'amount_display', x: 20, y: 332, width: 350, height: 36, zIndex: 1,
      content: { label: 'Total due today' },
    },
    {
      id: 'default-card', type: 'card_fields', x: 20, y: 380, width: 350, height: 130, zIndex: 1,
      content: {},
    },
    {
      id: 'default-pay', type: 'pay_button', x: 20, y: 522, width: 350, height: 50, zIndex: 1,
      content: { label: 'Continue to Payment', color: '#7c3aed' },
    },
  ];
}

/** Grows the canvas as blocks are placed further down — never clips content, always scrollable. */
export function computeCanvasHeight(blocks: PageBlock[]): number {
  return blocks.reduce((max, b) => Math.max(max, b.y + b.height), CANVAS_MIN_HEIGHT) + 40;
}

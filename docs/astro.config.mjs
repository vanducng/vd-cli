// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import react from '@astrojs/react';
import starlightLlmsTxt from 'starlight-llms-txt';
import remarkGfm from 'remark-gfm';

export default defineConfig({
  site: 'https://vd-cli.vanducng.dev',
  // GFM (tables, strikethrough) for MDX — .mdx does not get it by default.
  // NOTE: markdown.remarkPlugins is deprecated in Astro 6; migrate when bumping major.
  markdown: { remarkPlugins: [remarkGfm] },
  integrations: [
    starlight({
      title: 'vd-cli',
      logo: { src: './src/assets/logo.svg' },
      // Apply Starlight's markdown pipeline (asides, heading links) to the custom-loader content/ dir.
      markdown: { processedDirs: ['./content'] },
      description: 'A single-binary vendoring package manager for coding-agent skills.',
      customCss: ['./src/styles/theme.css'],
      expressiveCode: {
        themes: ['catppuccin-mocha', 'catppuccin-latte'],
        styleOverrides: { borderRadius: '0.5rem' },
      },
      components: {
        ThemeSelect: './src/components/ThemeSelect.astro',
        SocialIcons: './src/components/SocialIcons.astro',
        Search: './src/components/Search.astro',
      },
      plugins: [
        starlightLlmsTxt({
          projectName: 'vd-cli',
          description: 'A single-binary vendoring package manager for coding-agent skills.',
        }),
      ],
      lastUpdated: true,
      sidebar: [
        { label: 'Overview', link: '/' },
        { label: 'Guide', items: ['usage'] },
        { label: 'Reference', items: ['commands', 'config-schema'] },
        { label: 'Help', items: ['migration', 'faq'] },
        {
          label: 'Related docs',
          items: [
            { label: 'dotfiles', link: 'https://dotfiles.vanducng.dev', attrs: { target: '_blank' } },
            { label: 'skills', link: 'https://skills.vanducng.dev', attrs: { target: '_blank' } },
            { label: 'miu-db', link: 'https://db.miu.sh', attrs: { target: '_blank' } },
          ],
        },
      ],
    }),
    react(),
  ],
});

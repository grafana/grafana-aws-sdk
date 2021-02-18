import resolve from '@rollup/plugin-node-resolve';
import commonjs from '@rollup/plugin-commonjs';
import { terser } from 'rollup-plugin-terser';

import typescript from 'rollup-plugin-typescript2';

const pkg = require('./package.json');

const libraryName = pkg.name;

const buildCjsPackage = ({ env }) => {
  return {
    input: 'src/index.ts',
    output: [
      {
        file: `dist/index.${env}.js`,
        name: libraryName,
        format: 'cjs',
        sourcemap: true,
        chunkFileNames: `[name].${env}.js`,
        strict: false,
        exports: 'named',
        globals: {
          react: 'React',
          'prop-types': 'PropTypes',
        },
      },
    ],
    plugins: [
      typescript({
        rollupCommonJSResolveHack: false,
        clean: true,
      }),
      commonjs({
        include: /node_modules/,
      }),
      resolve(),
      env === 'production' && terser(),
    ],
    external: ['react', 'react-dom', '@grafana/data', '@grafana/ui', '@grafana/runtime'],
  };
};
export default [buildCjsPackage({ env: 'development' }), buildCjsPackage({ env: 'production' })];

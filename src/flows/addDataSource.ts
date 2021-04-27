import { e2e } from '@grafana/e2e';

import { E2ESelectors } from '@grafana/e2e-selectors';

export const Components = {
  ConfigEditor: {
    SecretKey: {
      input: 'Config editor secret key input',
    },
    AccessKey: {
      input: 'Config editor access key input',
    },
  },
};

export const selectors: { components: E2ESelectors<typeof Components> } = {
  components: Components,
};

const e2eSelectors = e2e.getSelectors(selectors.components);

export const addDataSourceWithKey = (
  datasourceType: string,
  accessKey: string,
  secretKey: string,
  region: string
): any => {
  return e2e.flows.addDataSource({
    checkHealth: false,
    expectedAlertMessage: 'Connection success',
    form: () => {
      setSelectValue('.aws-config-authType', 'Access & secret key');
      e2eSelectors.ConfigEditor.AccessKey.input().type(accessKey);
      e2eSelectors.ConfigEditor.SecretKey.input().type(secretKey);
      setSelectValue('.aws-config-defaultRegion', region);
    },
    type: datasourceType,
  });
};

const setSelectValue = (container: string, text: string) => {
  // return e2e.flows.selectOption({
  //   clickToOpen: true,
  //   optionText: text,
  //   container: e2e().get(container),
  // });

  // couldn't get above code to work for some reason. need to investigate that
  return e2e().get(container).parent().find(`input`).click({ force: true }).type(text).type('{enter}');
};

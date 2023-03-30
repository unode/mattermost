// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {connect} from 'react-redux';
import {bindActionCreators, Dispatch, ActionCreatorsMapObject} from 'redux';
import {Stripe} from '@stripe/stripe-js';

import {getAdminAnalytics} from 'mattermost-redux/selectors/entities/admin';
import {getClientConfig} from 'mattermost-redux/actions/general';
import {getCloudProducts, getCloudSubscription, getInvoices} from 'mattermost-redux/actions/cloud';
import {Action} from 'mattermost-redux/types/actions';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getTheme} from 'mattermost-redux/selectors/entities/preferences';

import {makeAsyncComponent} from 'components/async_load';

import {GlobalState} from 'types/store';
import {BillingDetails} from 'types/cloud/sku';

import {isModalOpen} from 'selectors/views/modals';
import {getCloudDelinquentInvoices, isCloudDelinquencyGreaterThan90Days} from 'selectors/cloud';
import {isDevModeEnabled} from 'selectors/general';

import {ModalIdentifiers} from 'utils/constants';

import {closeModal, openModal} from 'actions/views/modals';
import {completeStripeAddPaymentMethod, subscribeCloudSubscription} from 'actions/cloud';
import {ModalData} from 'types/actions';
import withGetCloudSubscription from 'components/common/hocs/cloud/with_get_cloud_subscription';
import {findOnlyYearlyProducts} from 'utils/products';
import {getCloudContactSalesLink, getCloudSupportLink} from 'utils/contact_support_sales';

const PurchaseModal = makeAsyncComponent('PurchaseModal', React.lazy(() => import('./purchase_modal')));

function mapStateToProps(state: GlobalState) {
    const subscription = state.entities.cloud.subscription;

    const isDelinquencyModal = Boolean(state.entities.cloud.subscription?.delinquent_since);
    const products = state.entities.cloud!.products;
    const yearlyProducts = findOnlyYearlyProducts(products || {});

    const customer = state.entities.cloud.customer;
    const customerEmail = customer?.email || '';
    const firstName = customer?.contact_first_name || '';
    const lastName = customer?.contact_last_name || '';
    const companyName = customer?.name || '';
    const contactSalesLink = getCloudContactSalesLink(firstName, lastName, companyName, customerEmail, 'mattermost', 'in-product-cloud');
    const contactSupportLink = getCloudSupportLink(customerEmail, 'Cloud purchase', '', window.location.host);

    return {
        show: isModalOpen(state, ModalIdentifiers.CLOUD_PURCHASE),
        products,
        yearlyProducts,
        isDevMode: isDevModeEnabled(state),
        contactSupportLink,
        invoices: getCloudDelinquentInvoices(state),
        isCloudDelinquencyGreaterThan90Days: isCloudDelinquencyGreaterThan90Days(state),
        isFreeTrial: subscription?.is_free_trial === 'true',
        isComplianceBlocked: subscription?.compliance_blocked === 'true',
        contactSalesLink,
        productId: subscription?.product_id,
        customer,
        currentTeam: getCurrentTeam(state),
        theme: getTheme(state),
        isDelinquencyModal,
        usersCount: Number(getAdminAnalytics(state)!.TOTAL_USERS) || 1,
    };
}
type Actions = {
    closeModal: () => void;
    openModal: <P>(modalData: ModalData<P>) => void;
    getCloudProducts: () => void;
    completeStripeAddPaymentMethod: (stripe: Stripe, billingDetails: BillingDetails, isDevMode: boolean) => Promise<boolean | null>;
    subscribeCloudSubscription: typeof subscribeCloudSubscription;
    getClientConfig: () => void;
    getCloudSubscription: () => void;
    getInvoices: () => void;
}

function mapDispatchToProps(dispatch: Dispatch) {
    return {
        actions: bindActionCreators<ActionCreatorsMapObject<Action>, Actions>(
            {
                closeModal: () => closeModal(ModalIdentifiers.CLOUD_PURCHASE),
                openModal,
                getCloudProducts,
                completeStripeAddPaymentMethod,
                subscribeCloudSubscription,
                getClientConfig,
                getInvoices,
                getCloudSubscription,
            },
            dispatch,
        ),
    };
}

export default connect(mapStateToProps, mapDispatchToProps)(withGetCloudSubscription(PurchaseModal));

// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage, useIntl} from 'react-intl';

import {useMeasurePunchouts} from '@mattermost/components';

import OnboardingWorkTemplateTourTip from './worktemplate_explore_tour_tip';
import {useShowTourTip} from './useShowTourTip';

export const BoardsTourTip = (): JSX.Element | null => {
    const {formatMessage} = useIntl();

    const {playbooksCount, boardsCount, showBoardsTour} = useShowTourTip();
    const overlayPunchOut = useMeasurePunchouts(['sidebar-right'], []);

    if (!showBoardsTour) {
        return null;
    }

    const title = (
        <FormattedMessage
            id='pluggable_rhs.tourtip.boards.title'
            defaultMessage={'Access your {count} linked {count, plural, one {board} other {boards}}!'}
            values={{count: boardsCount === 0 ? undefined : String(boardsCount)}}
        />
    );

    const screen = (
        <ul>
            <li>
                {formatMessage({
                    id: 'pluggable_rhs.tourtip.boards.access',
                    defaultMessage: 'Access your linked boards from the Boards icon on the right hand App bar.',
                })}
            </li>
            <li>
                {formatMessage({
                    id: 'pluggable_rhs.tourtip.boards.click',
                    defaultMessage: 'Click into boards from this right panel.',
                })}
            </li>
            <li>
                {formatMessage({
                    id: 'pluggable_rhs.tourtip.boards.review',
                    defaultMessage: 'Review boards updates from your channels.',
                })}
            </li>
        </ul>
    );

    return (
        <OnboardingWorkTemplateTourTip
            pulsatingDotPlacement={'left'}
            pulsatingDotTranslate={{x: 10, y: -350}}
            title={title}
            screen={screen}
            overlayPunchOut={overlayPunchOut}
            singleTip={playbooksCount === 0}
            placement='left-start'
            showOptOut={false}
        />
    );
};


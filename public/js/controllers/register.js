scionApp
    .controller('registerCtrl', ['$scope', 'registerService', 'ResolveSiteKey', '$interval',
        '$location', 'vcRecaptchaService',
        function ($scope, registerService, ResolveSiteKey, $interval, $location, vcRecaptchaService)
        {
            $scope.user = {};
            if($scope.siteKey == null) {
                $scope.siteKey = ResolveSiteKey.data;
            }

            $scope.register = function (user) {
                if (!$scope.user.captcha) {
                    $scope.error = "Please resolve the captcha before submitting.";
                    $scope.message = "";
                } else if (!$scope.registrationForm.$valid) {
                    $scope.error = "Please fill out the form correctly."
                } else {
                    registerService.register(user).then(
                        function (data) {
                            $scope.message = "Registration completed successfully. We sent you " +
                                "an email to your inbox with a link to verify your account.";
                            $scope.error = "";
                            $scope.user = {};
                            $scope.registrationForm.$setPristine();
                            vcRecaptchaService.reload();
                        },
                        function (response) {
                            $scope.error = response.data;
                            $scope.message = "";
                            vcRecaptchaService.reload();
                            console.log(response);
                        });
                }
            };

            $scope.dismissSuccess = function () {
                $scope.message = "";
            };

            $scope.dismissError = function () {
                $scope.error = "";
            };
        }
    ]);

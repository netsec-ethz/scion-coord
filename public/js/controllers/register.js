angular.module('scionApp')
    .controller('registerCtrl', ['$scope', 'ResolveSiteKey', '$interval', '$location', 'vcRecaptchaService',
        function($scope, ResolveSiteKey, $interval, $location, vcRecaptchaService) {

            $scope.user = {};
            $scope.siteKey = ResolveSiteKey.data;

            $scope.register = function(user) {

                if (!$scope.user.captcha){
                    $scope.error = "Please resolve the captcha before submitting."
                    $scope.message = "";
                } else {

                registerService.register(user).then(
                    function(data) {
                        $scope.message = "Registration completed successfully. We sent you an email to your inbox with a link to verify your account.";
                        $scope.error = ""
                        $scope.user = {};
                        vcRecaptchaService.reload();
                    },
                    function(response) {
                        $scope.error = response.data;
                        $scope.message = ""
                        vcRecaptchaService.reload();
                        console.log(response);
                    });
                }
            };

        }
    ]);
